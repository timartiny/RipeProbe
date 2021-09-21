# Query List

This script will take in results of scanning the Tranco top 1 million list using
[zdns](https://github.com/zmap/zdns) for both v4 and v6 addresses and using
[zgrab](https://github.com/zmap/zgrab2) for TLS support.

```
Usage:
  querylist [OPTIONS]

Application Options:
      --v4_dns=                Path to the ZDNS results for v4 lookups
      --v6_dns=                Path to the ZDNS results for v6 lookups
      --v4_tls=                Path to the ZGrab results for v4 TLS banner grabs
      --v6_tls=                Path to the ZGrab results for v6 TLS banner grabs
      --citizen_lab_directory= Path to the directory containing the Citizen Lab lists
      --out_file=              File to write all details to (in JSON)

Help Options:
  -h, --help                   Show this help message
```

This script will call out unusual circumstances, such as when a domain has
multiple v4 addresses and only supports TLS on some of them (and not all or
none).

All of the Application Options are required, a sample usage is:

`./querylist --v4_dns ../../data/v4-top-1m-sept-15.json --v6_dns ../../data/v6-top-1m-sept-15.json --v4_tls ../../data/v4-tls-top-1m-sept-15.json --v6_tls ../../data/v6-tls-top-1m-sept-15.json --citizen_lab_directory ../../../test-lists/lists/ --out_file ../../data/full-details-sept-15.json`

The output file will not be sorted by Tranco Rank, probably. To sort it and save
the results do:

`cat ../../data/full-details-sept-15.json | jq -s "sort_by(.tranco_rank) | .[]" -c > ../../data/full-details-sept-15-sorted.json`