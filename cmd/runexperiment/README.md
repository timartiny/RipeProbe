# run experiment
This directory contains the main module that will use the
[ripeexperiment](../../ripeexperiment) module to create a measurement by first
looking up probes in a given country, then determining which domains to issue A
and AAAA record requests for.

Usage:
```
Usage of ./runexperiment:
  -apiKey string
        API key as string
  -c string
        The Country Code to request probes from
  -intersectsize int
        Desired size of the intersection, defaults to 10 (default 50)
  -noatlas
        Will stop script from looking up v6 addresses from RIPE Atlas,
        using probe list
  -nointersect
        Will stop script from intersecting CitizenLab and Tranco CSV
        files. Future steps will assume intersection.csv exists
  -nolookup
        Will stop script from looking up whether intersection file has v6
        addresses. Future steps will assume lookup.csv exists
  -noprobe
        Will stop script from looking for probes from given country
```