package main

var procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")

// dpiAwarenessPerMonitorV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4).
const dpiAwarenessPerMonitorV2 = ^uintptr(3)

// matchWindowDPIAwareness makes the calling thread per-monitor DPI aware,
// as Gio makes its window thread. The executable declares no DPI awareness,
// so any other thread is DPI unaware and Windows scales the window
// coordinates it reads down by the monitor scale. ANGLE sizes the back
// buffer from GetClientRect on the thread that swaps: on a 125% monitor it
// came out at 80% of FrameEvent.Size, Gio drew the full frame into it
// anchored at the bottom left, and the window showed the UI enlarged with
// its top and right edges cut off. Call it on the locked thread that owns
// the EGL context, before the first GL call.
func matchWindowDPIAwareness() {
	procSetThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
}
