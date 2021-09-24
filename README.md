# RipeProbe
Repo for running a RIPE Atlas experiment to collect IPv{4,6} addresses for
desired domains from specific probes and countries.

See [cmd](cmd/) directory for executables and [ripeexperiment](ripeexperiment)
for libraries.

# Workflow

The goal of this repo is to run *The Whiteboard Experiment* which will choose
probes in non-censored countries from RIPE Atlas and use resolvers in censored
countries to resolve censored and uncensored domains.

There are many steps to this process which this document will present in order.

All of the tools for this workflow can be made with 

```bash
make
```

# Setup

The following steps need to be run one time to set up and organize all the data
necessary for future steps that will run for each country.

## Get Datafiles

We assume that the `top-1m.csv` file exits in the `data/` directory from Tranco.
We'll need to run `zdns` on this file. We use 4 different recursive name servers
(Google and Cloudflare) to resolve these domains for A and AAAA records.

```
cat data/top-1m.csv | ./zdns A --alexa --name-servers=8.8.8.8,8.8.4.4,1.1.1.1,1.0.0.1 --output-file data/v4-top-1m-<date>.json
cat data/top-1m.csv | ./zdns AAAA --alexa --name-servers=8.8.8.8,8.8.4.4,1.1.1.1,1.0.0.1 --output-file data/v6-top-1m-<date>.json
```

If the list you want to run scans on is not an Alexa formatted CSV file then
drop the `--alexa` flag and `zdns` will accept a list of domains.

Those are recursive resolvers, but the results will include CNAME records. We
also need to try each provided IP for a TLS cert for the provided domain. To get
the associated IP with each domain (excluding the CNAME intermediate steps, but
including the final mapping) we run a few complicated `jq` commands:

```
cat data/v4-top-1m-<date>.json | jq -r '.name as $name | .data.answers[]? | select(.type=="A") | "\(.answer), \($name)"' > data/v4-top-1m-ip-dom-pair-<date>.dat
cat data/v6-top-1m-<date>.json | jq -r '.name as $name | .data.answers[]? | select(.type=="AAAA") | "\(.answer), \($name)"' > data/v6-top-1m-ip-dom-pair-<date>.dat
```

Now data is in ip, domain pair lists that can be passed to ZGrab2 to get TLS certs

```
cat data/v4-top-1m-ip-dom-pair-<date>.dat | ./zgrab2 -o data/v4-tls-top-1m-<date>.json tls
cat data/v6-top-1m-ip-dom-pair-<date>.dat | ./zgrab2 -o data/v6-tls-top-1m-<date>.json tls
```
The first command took around 18 minutes on `zbuff`.

Those two commands will take the longest, collecting ~23 GB of data. Zgrab2
will probably need `sudo` access to send TCP packets.

## Run querylist

With v{4,6} DNS and TLS data we can now organize all the domains into a struct
that keeps track of:

* The domain
* The domain's rank
* Whether the domain has a v4 address
* Whether the domain has a v6 address
* Whether the domain supports TLS on all of its v4 addresses
* Whether the domain supports TLS on all of its v6 addresses

Querylist will group this information together. It will call out unusual
circumstances that might pop up (like if a domain supports TLS on some v{4,6}
addresses but not all).

To do so run `./querylist` a sample usage is:

`./querylist --v4_dns data/v4-top-1m-sept-15.json --v6_dns data/v6-top-1m-sept-15.json --v4_tls data/v4-tls-top-1m-sept-15.json --v6_tls data/v6-tls-top-1m-sept-15.json --citizen_lab_directory ../test-lists/lists/ --out_file data/full-details-sept-15.json`

This resulting file `data/full-details-sept-15.json` is not sorted by anything.
To sort by Tranco Rank, run:

`cat data/full-details-sept-15.json | jq -s "sort_by(.tranco_rank) | .[]" -c > data/full-details-sept-15-sorted.json`

## Find Open Resolvers

There are two types of resolvers in this experiment, open resolvers (i.e. actual resolvers online) and domains hosted in a country that are NOT actually resolvers. 

The domains (used as resolvers) will help us detect bi-directional censorship.

To generate the list of open resolvers:

1. Get a list of all IPv4 addresses that listen on port 53 (from Censys)
2. Host a domain that only has an AAAA record with a Name Server that you
   control (and only has a IPv6 address open to the public)
3. On the box that runs the Name Server, run tcpdump recording all requests.
4. Run the ./probe script (not in repo) which will take all of the IPv4
   resolvers (from Step 1) and make a AAAA record request for `<v4-ip>.<domain>`
5. Take the PCAP from the box running the Name Server and extract which IPv6
   addresses requested an AAAA record for which IPv4 address (from encoded
   request in Step 4.)
6. Make a list of single resolvers (there will be a lot of IPv4 addresses that
   use the same IPv6 resolver, so choose one).
7. Get country listing for the single resolver pairs, should look like `<v6
   address> <v4 address> <ISO country code>`.
8. For each address (both v6 and v4) do an A record lookup for a domain that
   only has a single IPv4 address and an AAAA record lookup for a domain that
   only have a single IPv6 address.
9. Remove from the list all lines that contain an IP that either:
	* didn't respond
	* gave the wrong IPv4 address
	* gave the wrong IPV6 address
10. Save the results.

The lastest collection of IPs was generated on August 30, 2021, and is saved in
`data/aug-30-2-single-resolvers-country-sorted` (all resolvers, Step 7.) and
`data/aug-30-2-single-resolvers-country-correct-sorted` (All correct resolver
pairs, Step 10.)

## Probe Generator

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

```
Usage: probegenerator [--all_probes_file ALL_PROBES_FILE] --filtered_probes_file FILTERED_PROBES_FILE

Options:
  --all_probes_file ALL_PROBES_FILE
                         Path to save all the probes data to
  --filtered_probes_file FILTERED_PROBES_FILE
                         (Required) Path to save the probes from not censored countryes, alive, and from different ASNs to
  --help, -h             display this help and exit
```

Uses the [probes](../../probes) module and prints output in JSON format, one per
line.

# Country Experiments
Now all the set up is complete. Each of the following steps needs to be run
individually for each country being tested.

## Selecting Domains

From this list you will need to manually select certain domains of interest. You
will only want domains that support v4, v6, and TLS. Of those domains you will
want to look for domains you expect to be censored in the country, these will
become query domains, and domains that are not censored in the country, some of
these will be control query domains, others might become resolvers. 

A starting point would be running:

```
cat data/full-details-sept-15-sorted.json | jq "select(.has_v4==true and .has_v6==true)" -c > data/full-details-v4-and-v6-sept-15.json
```

To select domains that have both a `v4` and `v6` address. To reduce that to
domains that have TLS on `v4` and `v6` you run:

```
cat full-details-v4-and-v6-sept-15.json | jq "select(.has_v4_tls==true and .has_v6_tls==true)" -c > full-details-v4-and-v6-and-tls-sept-15.json
```

Of the domains not censored by the given country, you will want to determine
which are hosted in the given country to do so you will need to create a file
that has one domain per line to pass to `inCoutnryLookup` (below).

For later (Whiteboard Experiment) you'll also want to create a list of domains
that will be used in that experiment, one domain per line, probably some should
be uncensored (as placebos) and others should be censored, as test.

Then on to the next step:

## Run inCountryLookup

In order to determine which (uncensored) domains are hosted in the country we
will use RIPE Atlas measurements to perform DNS lookups for us. While we should
only have domains that support v4 and v6 we will perform both A and AAAA lookups
to ensure that both IPs are in the country. Run:

```bash
./inCountryLookup --apiKey <key> --country_code <country_code> --domain_file <file with domains, one per line> --ids_file <file to save ids to, one per line>
```

Complete usage is:

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

the `ids_file` will be a list of integers that correspond to Measurement Ids in
RIPE Atlas.

## Fetch results

After the RIPE Atlas measurements have completed you can fetch the results with
a simple bash script here:

```bash
./fetchMeasurementResults.sh -a <api_key> -f data/Ids-<timestamp>
```

This will create a sub-directory in the `data` directory based on measurement
IDs i.e., if the first measurment in the list is 30250495 and the last is
30250522 then it will create the directory `data/30250495-30250522/` to store
all the measurement results.

## Parse In Country Lookup Results

Next the results need to be merged back into the lookup file. Use the
`./parseInCountryLookup` script to do this.

```bash
./parseInCountryLookup --in data/<country_code>_lookup.json --out data/<country_code>-<date>_lookup.json --ids data/Ids-<timestamp>
```

## Make a list of resolvers
After the list of correct open resolvers is made you can run:
```
./resolverlist -c <country_code> --lookup <path_to_non_censored_domains> --out <path_to_save_resolvers> --resolvers <path_from_above>
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


## Whiteboard Experiment

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

## Fetch results (again)

No change here but this time you run:

```bash
./fetchMeasurementResults.sh -a <api_key> -f data/Whiteboard-Ids-<country_code>-<timestamp>
```

This will create a subdirectory in the `data` directory such as
`data/30251621-30251733/`

## Parse Whiteboard Experiment

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

The IP, Domain pairs are exactly what they sound like:

```
31.13.83.2, dailymotion.com
157.240.17.36, dailymotion.com
103.240.182.55, dailymotion.com
2001:0:0:0:0:0:1f0d:5011, dailymotion.com
2001:0:0:0:0:0:1f0d:5111, dailymotion.com
2001:0:0:0:0:0:629f:6c3d, dailymotion.com
162.125.32.2, dailymotion.com
202.160.130.145, dailymotion.com
199.96.62.75, dailymotion.com
2001:0:0:0:0:0:2f58:3aea, dailymotion.com
...
```

## Verify the IP, Domain results

Use Zgrab2 to get information on the IP, Domain pairings, run:

`cat ip_dom_pairs | zgrab2 -o tls_ip_dom_pairs.json tls`


# This is extraneous stuff at the moment
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
