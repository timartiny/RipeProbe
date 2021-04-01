# Query List

This script will construct a list of domains to do DNS lookups for. It will find
popular domains that are blocked and one that is unblocked (as a control.)

This assumes particular structures for popularity files (namely Tranco list
format) and blocked files (namely citizenlab list format).

Usage:
```
./querylist -p <file with popular domains> -b <file with blocked domains> [-n <number of query domains>] -c <country code>
```