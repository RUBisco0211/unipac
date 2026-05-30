import { EventsOn, EventsOff } from "@/../wailsjs/runtime/runtime";
import { onMounted, onUnmounted } from "vue";

export function useWailsEvent(name:string, callback:(...args:any[]) => void) {
    onMounted(() => {
        EventsOn(name, () => {
            callback()
        })
    })
    onUnmounted(() => {
        EventsOff(name)
    })
    
}

