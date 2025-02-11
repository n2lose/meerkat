package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/mailgun/groupcache/v2"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

func initDirs() error {
	requiredDirs := []string{"dashboards", "dashboards-data"}

	files, err := ioutil.ReadDir("./")
	if err != nil {
		return fmt.Errorf("Failed to read directory - %s", err)
	}

	//Check directories exist, if not create
Files:
	for _, p := range requiredDirs {
		for _, f := range files {
			if f.Name() == p {
				continue Files
			}
		}

		fmt.Printf("%s directory not found, creating\n", p)
		err = os.Mkdir(p, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}

type Config struct {
	HTTPAddr string

	IcingaURL         string
	IcingaUsername    string
	IcingaPassword    string
	IcingaInsecureTLS bool

	AdminUsername string
	AdminPassword string

	CacheExpiryDurationSeconds int64
	CacheSizeBytes             int64
}

var config Config

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "./meerkat.toml", "provide an alternative config path")
	flag.Parse()

	//Config
	fmt.Printf("Reading config: %s\n", configFile)
	_, err := toml.DecodeFile(configFile, &config)
	if err != nil {
		log.Fatalf("Error reading config - %s\n", err)
	}

	if config.CacheExpiryDurationSeconds == 0 {
		config.CacheExpiryDurationSeconds = 16
	}
	if config.CacheSizeBytes == 0 {
		config.CacheSizeBytes = 20971520
	}

	initialiseIcingaCaches()

	//Initialize directories
	err = initDirs()
	if err != nil {
		log.Fatalf("Error initializing dashboards directory - %s\n", err)
	}

	cachepool := groupcache.NewHTTPPool("http://localhost:8585")
	cachepool.Set("http://localhost:8585")

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Handle("/_groupcache/", cachepool)

	r.Get("/dashboard", handleListDashboards)
	r.Get("/dashboard/{slug}", handleListDashboard)

	var (
		createDashboard http.HandlerFunc
		updateDashboard http.HandlerFunc
		deleteDashboard http.HandlerFunc
	)
	if config.AdminUsername != "" && config.AdminPassword != "" {
		createDashboard = basicAuthHandler(config.AdminUsername, config.AdminPassword, http.HandlerFunc(handleCreateDashboard)).ServeHTTP
		updateDashboard = basicAuthHandler(config.AdminUsername, config.AdminPassword, http.HandlerFunc(handleUpdateDashboard)).ServeHTTP
		deleteDashboard = basicAuthHandler(config.AdminUsername, config.AdminPassword, http.HandlerFunc(handleDeleteDashboard)).ServeHTTP
	} else {
		createDashboard = handleCreateDashboard
		updateDashboard = handleUpdateDashboard
		deleteDashboard = handleDeleteDashboard
	}
	r.Post("/dashboard", createDashboard)
	r.Post("/dashboard/{slug}", updateDashboard)
	r.Delete("/dashboard/{slug}", deleteDashboard)

	// settings - custom branding, white labeling
	r.Get("/settings", handleGetSettings)
	r.Post("/settings", handleChangeSettings)

	// relay to icinga server
	// browser <-> meerkat <-> icinga
	r.Get("/icinga/{check-type}", handleIcingaCheck)
	r.Get("/icinga/dynamic_text/{host-name}", handleIcingaVars)
	r.Get("/icinga/{check-type}/{object-id}", handleIcingaCheck)
	r.Get("/icinga/check_state", handleIcingaCheckState)
	r.Get("/icinga/check_result", handleCheckResult)

	// find and return dashboard json (useful for debugging),
	// e.g. GET /template?templateid=ha.json
	// would stream back content of /path/to/meerkat/root/dashboards/ha.json
	r.Get("/template", handleCreateTemplate)

	// persist user uploaded files onto server's local filesystem
	r.Post("/upload", handleUpload)

	// serve static assets, referenced in dashboard json configurations
	r.Handle("/dashboards-data/*", http.StripPrefix("/dashboards-data/", http.FileServer(http.Dir("./dashboards-data"))))

	// serve frontend stuff
	// otherwise, 404 when nothing is found
	r.NotFound(createFileHandler("./frontend"))

	fmt.Printf("Starting web server: %s\n", config.HTTPAddr)
	log.Fatal(http.ListenAndServe(config.HTTPAddr, r))
}

//Serves index.html if the path isn't found
func createFileHandler(frontendPath string) func(w http.ResponseWriter, r *http.Request) {
	frontendDir := http.Dir(frontendPath)
	fs := http.FileServer(frontendDir)

	return func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		f, err := frontendDir.Open(p)

		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Open(path.Join(frontendPath, "index.html"))
			if err != nil {
				http.Error(w, "Error opening index.html - "+err.Error(), http.StatusInternalServerError)
			}
			_, err = io.Copy(w, f)
			if err != nil {
				http.Error(w, "Error writing index.html to response - "+err.Error(), http.StatusInternalServerError)
			}
			f.Close()
		} else {
			f.Close()
			fs.ServeHTTP(w, r)
		}
	}
}
