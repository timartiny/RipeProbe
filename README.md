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

All of the tools for this workflow can be made with 

```bash
make
```

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

This step is not implemented yet, but should be in the following steps:

1. Create a list of domains that are hosted in the country, using inCountryLookup through parseresults above.
2. Choose sufficient number of both v4 and v6 addresses.
3. Get a list of Open Resolvers (both v4 and v6 addresses) hosted in the country
4. Get a list of uncensored domains that have v4 and v6 addresses by the country (could be list above, should be about 5 domains)
5. For each resolver IP run [ZDNS](https://github.com/zmap/zdns) using it as a resolver for each of the domains from step 4.
6. Verify each resolved domain A and AAAA record (using [ZGrab](https://github.com/zmap/zgrab2)'s TLS module (might need to locally verify certs).
7. Include sufficient number of Open Resolvers in the country that provide valid v4 and v6 addresses for uncensored domains.

### Probe Generator

This script will find all RIPE Atlas probes that are not in our list of censored
countries:

* China
* Iran
* Russia
* Saudi Arabia
* South Korea
* India
* Pakistan
* Egypt
* Argentina
* Brazil

Has been connected for at least a week, has a v4 and v6 address and does not
share a v4 or v6 ASN with any other probe. This list might have over 1000 probes
and might need to be filtered further.

```bash
./probegenerator
```

### Whiteboard Experiment

Once all the probes are selected, the resolvers are chosen, and the domains are
listed we can run the white board experiment. Run:

```bash
./whiteboard --apiKey <api_key> -c <country_code> -p <path_to_probe_ids> -r <path_to_resolvers_file> -q <path_to_query_domains>
```

This will schedule numerous RIPE Atlas measurments. Note that the script will
follow the no more than 100 concurrent measurment rate-limit but does not check
others. Measurments may be scheduled then not run.

All the measurment IDs are saved in
`data/Whiteboard-Ids-<country_code>-<timestamp>`.

### Fetch results (again)

No change here but this time you run:

```bash
./fetchMeasurementResults.sh -a <api_key> -f data/Whiteboard-Ids-<country_code>-<timestamp>
```

This will create a subdirectory in the `data` directory such as
`data/30251621-30251733/`

### Whiteboard Results

To get *The Whiteboard Experiment* results into a form we can use we run:

```bash
./whiteboardresults -m data/Whiteboard-Ids-<country_code>-<timestamp> -r data/<country_code>_resolver_ips.dat
```

This will create
`data/<measurement_id1>-<measurement_id2>/Whiteboard_results<measurement_id1>-<measurement_id2>.json`

### v4 vs v6

Once the Whiteboard Results are collated into a file, we can compare the results between requests for v4 and v6 addresses. This script will print the results:

```bash
./v4vsv6 -r data/<meas_id1>-<meas_id2>/Whiteboard_results<meas_id1>-<meas_id2>.json -u "<string of comma separated domains to considered 'unblocked'>"
```

## Querylist

In order to determine which domains might be interesting to scan for we use
the `querylist` program, written in [the querylist directory](cmd/querylist).

It has a README on usage which can be applied to the executable in this
directory.

## InCountryLookup

To use domains hosted in a country as resolvers we need to know the IPs
associated with the domains. `inCountryLookup` will do this when specified with
a country. More documentation can be found in [the runexperiment
directory](cmd/runexperiment).

## ParseResults

This will combine the brief file manually created that lists domains in JSON
with the results of RIPE Atlas Measurements in order to create a resolver list.

More info can be found in [the parseresults directory](cmd/parseresults).

## Whiteboard

This will run the goal of this repo, *The Whiteboard Experiment*. 

More info can be found in [the whiteboard directory](cmd/whiteboard).

## Whiteboard Results

This will parse the RIPE Atlas measurment results into a JSON file of the
important bits. More info can be found in [the whiteboardresults
directory](cmd/whiteboardresults).

## v4 vs v6

This will look at the results of a Whitboard Experiment and print a table
summarizing the results breaking v4 requests separate from v6 requests.
