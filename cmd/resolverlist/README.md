# Resolver List

This script will create the simplified list of "resolvers" for the whiteboard experiment.

It assumes:

1. A query (`../../runexperiment`) has already been run to do DNS queries on popular domains for a particular country.

2. The results of that query have been fetched.

3. The results of that query have been merged together.

4. That a separate experiment has been run to determine correct open resolvers

Then this script will read those merged results, and store them in 
```
<ip_addr> <domain, or open>
```
form. This script will then use an ip->country database
(`../../data/geolite-country.mmdb`) to lookup the country for each IP. If the
country matches the provided country code it will save it and toss it if not.

Finally, if a separate list of resolvers (having the form `<v6 addr> <v4 addr>
<country_code>`) is provided then this script will add 5 lines of address (so 10
total addresses) 

Script will write results (in form above, with resolvers getting a domain of
`<country_code>_Resolver`) to provided file.

```
Usage of ./resolverlist:
  -c string
        Country code to check IPs against
  -lookup string
        Path to JSON file that has measurement data
  -out string
        Path to write resolver list to
  -resolvers string
        Path to file containing open resolvers that are assumed to be correct, with country code
```