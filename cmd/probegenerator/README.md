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