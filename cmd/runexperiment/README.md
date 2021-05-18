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

## Use in overall experiment

This code will look for probes in a given country then use those probes to
resolve domains.

As such it is more useful in our overall experiment for finding the IPs of
domains that are hosted in the country that can be used as "domain resolvers" in
the whiteboard experiment.

While there is more capability with this script due to early visions for the experiment, the current usage is usually simpler:

`./inCountryLookup --apiKey <key> -c <country_code> --nointersect --noExtraDomains --nolookup`

and assumes that the domains to lookup are in a file `data/<country_code>_lookup.json` with the form of 

```json
[{"domain":<domain_name>},{"domain":<domain_name_2>}]
```