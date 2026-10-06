# IMG-MUT

Mutation-testing base (SC-005). Contains the IMG-GO toolchain only: the mutation tools are added by their tasks after the 11.4.270 existence verdict of each (T479 go-mutesting fork, T480 Stryker, T481 PIT and cargo-mutants) as pinned layers of `Containerfile`.
Built from this directory; the first users are WP-61 through `scripts/containers/run_pinned.sh IMG-MUT -- <command>`. A dedicated runner `scripts/containers/run_mut.sh` is a tracked item owned by WP-61, not by this task.
Honest limit: until T479 to T481 land, a mutation run in this image has no mutation tool: no score may be recorded from it.
