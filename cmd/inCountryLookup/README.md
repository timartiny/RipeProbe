# inCountryLookup
This directory contains the main module that will create RIPE Atlas measurements
by first looking up probes in a given country, then issue A and AAAA record
requests for provided domains.

```
Usage:
  inCountryLookup [OPTIONS]

Application Options:
      --country_code= (Required) The Country Code to request probes from
      --domain_file=  (Required) Path to the file containing the domains to perform DNS lookups for, one domain per line
      --api_key=      (Required) Quote enclosed RIPE Atlas API key
      --ids_file=     (Required) Path to the file to write the RIPE Atlas measurement IDs to
      --get_probes    Whether to get new probes or not. If yes and probes_file is specified the probe ids will be written there
      --probes_file=  If get_probes is specified this is the file to write out the probes used in this experiment if get_probes is not specified then this is the file to read probes from. If ommitted nothing is
                      written
      --num_probes=   Number of probes to do lookup with (default: 5)

Help Options:
  -h, --help          Show this help message

```


This will schedule twice the number of domains in the `domain_file` experiments
(two experiments per domain, one to lookup A recored, one to lookup AAAA
record). The script will select `num_probes` random probes in the country
selected. The experiment Ids will be saved in `ids_file`.

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