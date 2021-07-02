import { h, Fragment } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import * as meerkat from '../meerkat';

export function HoverText({options, mouseOn, style}) {
    const [hostInfo, setHostInfo] = useState(null);

	useEffect(async () => {
        if (options.objectType !== null && options.filter !== null) {
            try {
                const res = await meerkat.getIcingaObjectInfo(options.objectType, options.id);
                let states = {};
                let stateInfo = "";

                res.results.forEach(result => {
                    if (!states.hasOwnProperty(result.attrs.state)) {
                        Object.assign(states, {[result.attrs.state]: 0});
                    }
                    if (states.hasOwnProperty(result.attrs.state)) {
                        states[result.attrs.state]++
                    }
                })

                for (const [key, value] of Object.entries(states)) {
                    if(options.objectType === 'host') {
                        switch(true){
                            case (key === '0' && value > 0):
                                stateInfo += `${value} OK, `;
                                break;
                            case (key === '1' && value > 0):
                                stateInfo += `${value} Warning, `;
                                break;
                            case (key === '2' && value > 0):
                                stateInfo += `${value} Critical, `;
                                break;
                            case (key === '3' && value > 0):
                                stateInfo += `${value} Unknown, `;
                                break;
                        }
                    } else if(options.objectType === 'host') {
                        switch(true){
                            case (key === '0' && value > 0):
                                stateInfo += `${value} Up, `;
                                break;
                            case (key === '1' && value > 0):
                                stateInfo += `${value} Down, `;
                                break;
                        }
                    }
                }

                setHostInfo([{childCount: `${res.results.length} ${res.results[0].type}s, ${stateInfo.slice(0, -2)}`},
                             {notes: `Notes from icinga test text`}]);
            } catch {
                console.log("There was an error retriving object data from icinga");
            }
        }
    }, [mouseOn]);

    if (!hostInfo) {
        return <div class="hover" style={style}>
        <p>Loading...</p>
    </div>
    }

    if (mouseOn) {
        return <div class="hover" style={style}>
            <p>{hostInfo[0].childCount}</p>
            <hr/>
            <p>{hostInfo[1].notes}</p>
        </div>
    }
}