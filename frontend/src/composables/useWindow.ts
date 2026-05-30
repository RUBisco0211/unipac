import {
    WindowFullscreen,
    WindowIsFullscreen,
    WindowIsMaximised,
    WindowMaximise,
    WindowToggleMaximise,
} from '../../wailsjs/runtime/runtime'

export const appWindow = {
    fullscreen: () => WindowFullscreen(),
    isFullscreen: () => WindowIsFullscreen(),
    maximize: () => WindowMaximise(),
    isMaximized: () => WindowIsMaximised(),
    toggleMaximize: () => WindowToggleMaximise(),
}
