# Probe Generator

This script will grab all RIPE Atlas probes that are currently alive and have v4
and v6 addresses.

It will then filter that list down to ones that have been Connected for at least
a week, ones that are not in our list of "censored" countries, and any that
share ASNs

Our working list of Censored countries:

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

Usage:

```bash
./probegenerator
```
It will write `data/uncensored_probes.dat` with format

`<probe id> <v4 ASN> <v6 ASN>`