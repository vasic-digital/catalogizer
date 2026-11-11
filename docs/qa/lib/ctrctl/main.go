// ctrctl: tiny QA lifecycle CLI over the Containers submodule's runtime abstraction
// (digital.vasic.containers/pkg/runtime, PodmanRuntime). Constitution 11.4.76 / 11.4.74.
//   ctrctl start|stop|status <container>
// Env hygiene (DBUS address without ,guid=) is done by the calling shell helper
// (docs/qa/lib/podman_env.sh) because the runtime inherits the process environment.
// status prints "<state> <health>" ("none" when no healthcheck).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"digital.vasic.containers/pkg/runtime"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: ctrctl start|stop|status <container>")
		os.Exit(64)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	rt := runtime.NewPodmanRuntime()
	id := os.Args[2]
	var err error
	switch os.Args[1] {
	case "start":
		err = rt.Start(ctx, id)
	case "stop":
		err = rt.Stop(ctx, id, runtime.WithStopTimeout(30*time.Second))
	case "status":
		var st *runtime.ContainerStatus
		if st, err = rt.Status(ctx, id); err == nil {
			h := st.Health
			if h == "" {
				h = "none"
			}
			fmt.Printf("%s %s\n", st.State, h)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(64)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ctrctl:", err)
		os.Exit(1)
	}
}
