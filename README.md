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

### Get Datafiles

We assume that the `top-1m.csv` file exits in the `data/` directory from Tranco.
We'll need to run `zdns` and `zgrab2` on this file:

```
cat data/top-1m.csv | ./zdns A --alexa --output-file /data/v4-top-1m.json
cat data/top-1m.csv | ./zdns AAAA --alexa --output-file /data/v6-top-1m.json
cat data/top-1m.csv | awk -F"," '{print $2}' | ./zgrab2 -o /data/tls-top-1m.json tls
```

### Run querylist

The first step is to collect info on popular domains in the world, filtered
through the Citizen Lab data. This will require `/data/v4-top-1m.json`,
`/data/v6-top-1m.json` and `/data/tls-top-1m.json`, at the very least the TLS
one should be up to day otherwise the script will say no website has TLS due to
out of date certificates.

```bash
./querylist  --v4 data/v4-top-1m.json --v6 data/v6-top-1m.json --tls data/tls-top-1m.json --cit-lab-global data/global.csv --cit-lab-country data/<country_code>.csv -c <country_code>
```

This will generate both `data/top-1m-tech-details.json` (which can be use for
future runs with other countries which can be used with the `--tech` flag instead
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

For later (Whiteboard Experiment) you'll also want to create a list of domains
that will be used in that experiment, one domain per line, probably some should
be uncensored (as placebos) and others should be censored, as test.

Then on to the next step:

### Run inCountryLookup

In order to determine which (uncensored) domains are hosted in the country we
will use RIPE Atlas measurements to perform DNS lookups for us. While we should
only have domains that support v4 and v6 we will perform both A and AAAA lookups
to ensure that both IPs are in the country. Run:

```bash
./inCountryLookup --apiKey <key> -c <country_code> 
```

This will gather all probes in the specified country that support v4 and v6
measurements and will randomly select 5 of them to do A and AAAA lookups with
those probes for the provided domains (saved in
`data/<country_code>_lookup.json`).

This will schedule a series of measurements with RIPE Atlas, and tell you the
time they will start. It will save the IDs of the measurements in the `data`
directory in a file `data/inCountryLookup-Ids-<timestamp>`. 

### Fetch results

After the RIPE Atlas measurements have completed you can fetch the results with
a simple bash script here:

```bash
./fetchMeasurementResults.sh -a <api_key> -f data/Ids-<timestamp>
```

This will create a sub-directory in the `data` directory based on measurement
IDs i.e., if the first measurment in the list is 30250495 and the last is
30250522 then it will create the directory `data/30250495-30250522/` to store
all the measurement results.

### Parse In Country Lookup Results

Next the results need to be merged back into the lookup file. Use the
`./parseInCountryLookup` script to do this.

```bash
./parseInCountryLookup --in data/<country_code>_lookup.json --out data/<country_code>-<date>_lookup.json --ids data/Ids-<timestamp>
```

### Resolver List

There are two types of resolvers in this experiment, open resolvers (i.e. actual resolvers online) and domains hosted in a country that are NOT actually resolvers. 

The domains (used as resolvers) will help us detect bi-directional censorship.

To generate the list of open resolvers:

1. Get a list of all IPv4 addresses that listen on port 53 (from Censys)
2. Host a domain that only has an AAAA record with a Name Server that you control (and only has a IPv6 address open to the public)
3. On the box that runs the Name Server, run tcpdump recording all requests.
4. Run the ./probe script (not in repo) which will take all of the IPv4 resolvers (from Step 1) and make a AAAA record request for <v4-ip>.<domain>
5. Take the PCAP from the box running the Name Server and extract which IPv6 addresses requested an AAAA record for which IPv4 address (from encoded request in Step 4.)
6. Make a list of single resolvers (there will be a lot of IPv4 addresses that use the same IPv6 resolver, so choose one).
7. Get country listing for the single resolver pairs, should look like <v6 address> <v4 address> <ISO country code>.
8. For each address (both v6 and v4) do an A record lookup for a domain that only has a single IPv4 address and an AAAA record lookup for a domain that only have a single IPv6 address.
9. Remove from the list all lines that contain an IP that either:
	* didn't respond
	* gave the wrong IPv4 address
	* gave the wrong IPV6 address
10. Save the results.

The lastest collection of IPs was generated on August 30, 2021, and is saved in
`data/aug-30-2-single-resolvers-country-sorted` (all resolvers, Step 7.) and
`data/aug-30-2-single-resolvers-country-correct-sorted` (All correct resolver
pairs, Step 10.)

After the list of correct open resolvers is made you can run:
```
./resolverlist -c <country_code> --lookup <path_to_lookup_file> --out <path_to_save_resolvers> --resolvers <path_from_above>
```

Sample usage:
```
/resolverlist -c CN --lookup data/CN_lookup-sept-8-full.json --out data/CN_resolver_ips.dat --resolvers data/aug-30-2-single-resolvers-country-correct-sorted
```

You can filter this list to the lines that fall into unique ASNs and sort the
data by type of resolver with: 

```
./unique_asn.py data/GeoLite2-ASN.mmdb data/CN_resolver_ips.dat | sort -k 2 >
data/CN_resolver_ips_unique_asn_sorted.dat
```

You can sort the resolver ips by domain/openresolver using `sort -k 2
data/CN_resolver_ips.dat > data/tmp`

See [resolverlist directory](cmd/resolverlist) for more specific readme.

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

This will create `data/uncensored_probes.dat` with format 

`<probe id> <v4 ASN> <v6 ASN>`

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

### Parse Whiteboard Experiment

The raw results from RIPE Atlas are hard to read as retrieved, we only care about a subset of the data. To get the simplified results run

`./parse_whiteboard_experiment.py data/Whiteboard-Ids-<country_code>-<timestamp>
data/simplified_results_<country_code>_<timestamp>.json
data/ip_dom_pairs_<country_code>_<timestamp>`

The simplified results file will be a JSON file with two types of objects:

```
{
	"probe_id": 9,
	"had_error": false,
	"resolver": "103.110.80.16",
	"record_type": "AAAA",
	"domain": "t66y.com",
	"answers": [
		"2606:4700:20:0:0:0:ac43:4af1",
		"2606:4700:20:0:0:0:681a:aa0",
		"2606:4700:20:0:0:0:681a:ba0"
	]
}
```

or
```
{
    "probe_id": 38,
    "had_error": true,
    "error": "Timeout: 5000",
    "resolver": "159.226.6.133"
}
```

Where "error" only shows up if there was one, but the "error" might be different.

The IP Domain pairs are exactly what they sound like:

```
31.13.83.2 dailymotion.com
157.240.17.36 dailymotion.com
103.240.182.55 dailymotion.com
2001:0:0:0:0:0:1f0d:5011 dailymotion.com
2001:0:0:0:0:0:1f0d:5111 dailymotion.com
2001:0:0:0:0:0:629f:6c3d dailymotion.com
162.125.32.2 dailymotion.com
202.160.130.145 dailymotion.com
199.96.62.75 dailymotion.com
2001:0:0:0:0:0:2f58:3aea dailymotion.com
...
```

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
