# RipeProbe
Repo for running a RIPE Atlas experiment to collect IPv{4,6} addresses for
desired domains from specific probes and countries.

See [cmd](cmd/) directory for executables and [ripeexperiment](ripeexperiment)
for libraries.

## Workflow

The goal of this repo is to run *The Whiteboard Experiment* which will choose
probes in non-censored countries from RIPE Atlas and use resolvers in censored
countries to resolve censored and uncensored domains.

There are many steps to this process which this document will present in order.

### Run querylist

The first step is to collect info on popular domains in the world, filtered through the Citizen Lab data.

```bash
./querylist  --v4 data/v4-top-1m.json --v6 data/v6-top-1m.json --tls data/tls-top-1m.json/ --cit-lab-global data/global.csv --cit-lab-country data/<country_code>.csv -c <country_code>
```

This will generate both `data/top-1m-tech-details.json` (which can be use for
future runs with other countries which can be use with the `--tech` flag instead
of the `--v4, --v6, --tls` flags) and
`data/<country_code>-top-1m-ripe-ready.json` which will provide information on
which domains are of interest globally as well as to the provided country.

From this list (which can be sorted by rank) you will need to manually select
certain domains of interest. You will only want domains that support v4, v6, and
TLS. Of those domains you will want to look for domains you expect to be
censored in the country, these will become query domains, and domains that are
not censored in the country, some of these will be control query domains, others
might become resolvers. 

Of the domains not censored by the given country, you will want to determine
which are hosted in the given country to do so you will need to create a file in
the data directory: `data/<country_code>_lookup.json` (such as
`data/CN_lookup.json`) with the following format:

```json
[{"domain":<domain_name>},{"domain":<domain_name_2>}]
```

Then on to the next step:

### Run inCountryLookup

In order to determine which (uncensored) domains are hosted in the country we
will use RIPE Atlas measurements to perform DNS lookups for us. While we should
only have domains that support v4 and v6 we will perform both A and AAAA lookups
to ensure that both IPs are in the country. Run:

```bash
./inCountryLookup --apiKey <key> -c <country_code> --nointersect --noExtraDomains --nolookup
```

This will gather all probes in the specified country that support v4 and v6
measurements and will randomly select 5 of them to do A and AAAA lookups with
those probes for the provided domains (in `data/<country_code>_lookup.json`).

This will schedule a series of measurements with RIPE Atlas, and tell you the
time they will start. It will save the IDs of the measurements in the `data`
directory in a file `data/Ids-<timestamp>`. 

### Fetch results

After the RIPE Atlas measurements have completed you can fetch the results with
a simple bash script here:

```bash
./fetchMeasurementResults.sh -a <api_key> -f data/Ids-<timestamp>
```

This will create a sub-directory in the `data` directory based on the
measurement IDs i.e., if the first measurment in the list is 30250495 and the
last is 30250522 then it will create the directory `data/30250495-30250522/` to
store all the measurement results.

### Parse Results

Next the results need to be merged back into the lookup file. Use the
`./parseresults` script to do this.

```bash
./parseresults -f data/<country_code>_lookup.json --id data/Ids-<timestamp>
```

### Resolver List

Massive TODO

## Querylist

In order to determine which domains might be interesting to scan for we use
the `querylist` program, written in [the querylist directory](cmd/querylist).

It has a README on usage which can be applied to the executable in this
directory.

## InCountryLookup

To use domains hosted in a country as resolvers we need to know the IPs associated with the domains. `inCountryLookup` will do this when specified with a country. More documentation can be found in [the runexperiment directory](cmd/runexperiment).

## ParseResults

This will combine the brief file manually created that lists domains in JSON
with the results of RIPE Atlas Measurements in order to create a resolver list.

More info can be found in [the parseresults directory](cmd/parseresults).