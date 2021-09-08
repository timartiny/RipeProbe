# inCountryLookup
This directory contains the main module that will use the
[ripeexperiment](../../ripeexperiment) module to create a measurement by first
looking up probes in a given country, then determining which domains to issue A
and AAAA record requests for.

Usage:
```
Usage of ./inCountryLookup:
  -apiKey string
        API key as string
  -c string
        The Country Code to request probes from
  -lookupFile string
        JSON file containing domains to lookup with RIPE Atlas, otherwise uses Country Code for default file
  -noatlas
        Will just do a dry run, won't actually call out to RIPE Atlas
  -probeFile string
        JSON file of probes that can be provided instead of doing a live lookup
```

This will schedule twice the number of domains in the lookupFile experiments
(two experiments per domain, one to lookup A recored, one to lookup AAAA
record). The script will select 5 random probes in the country selected. The
experiment Ids will be saved in
`data/inCountryLookup-Ids-<year>-<month>-<day>::<hour>:<minute>` where the date
is the time the experiments will run.

## Use in overall experiment

This code will look for probes in a given country then use those probes to
resolve domains.

The following is sufficient to run this script if using the default lookup file
(`data/<country_code>_lookup.json`) and looking up probes: 

`./inCountryLookup --apiKey <key> -c <country_code>`

assumes that the domains to lookup are in a file
`data/<country_code>_lookup.json` with the form of 

```json
[{"domain":<domain_name>},{"domain":<domain_name_2>}]
```

You can also specify the lookup file or a list of probes with

`./inCountryLookup --apiKey <key> -c <country_code> --lookupFile <path_to_lookup_file> --probeFile <path_to_probe_file`