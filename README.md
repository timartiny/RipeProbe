# RipeProbe
Repo for running a RIPE Atlas experiment to collect IPv{4,6} addresses for
desired domains from specific probes and countries.

See [cmd](cmd/) directory for executables and [ripeexperiment](ripeexperiment)
for libraries.

## Querylist

In order to determine which domains might be interesting to scan for we use
the `querylist` program, written in [the querylist directory](cmd/querylist).

It has a README on usage which can be applied to the executable in this
directory.