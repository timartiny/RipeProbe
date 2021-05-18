# Whiteboard experiment

This script will run the "whiteboard" experiment:

* Grab specfied number of probes from not-censored countries
* Use list of provided resolvers (ip addresses)
* Ask RIPE Atlas to have probes ask those resolvers to do DNA A and AAAA lookups for a provided list of domains
* Save the measurement IDs in a file to be looked at later

Usage:
```
./whiteboard -c <country code> {-n <number of probes> | -p <path to file containing probe IDs>} -q <path to query domains> -r <path to resolver ips>
```