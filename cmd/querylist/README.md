# Query List

This script will take in results of scanning the Tranco top 1 million list using
[zdns](https://github.com/zmap/zdns) for both v4 and v6 addresses and using
[zgrab](https://github.com/zmap/zgrab2) for TLS support.

This will output a file `top-1m-tech-details.json` in the data folder.

If Citizen Lab lists are also provided it will check the provided global and
country specific lists against the Tranco domains and determine whether they are
in each list.

The results of this will be in `<country-code>-top-1m-ripe-ready.json` where
country code is also provided to the script.

Usage:

```bash
./querylist {--v4 v4_path --v6 v6_path --tls tls_path | --tech tech_path} [--cit-lab-global global_path --cit-lab-country country_path -c country_code]
```

Technical details must be provided to this script, either via `--v4, --v6, --tls` or via `--tech` (the output of running this script previously with `--v4, --v6, --tls`).

In order for this script to check against Citizen Lab lists the path to the
lists must be provided, as well as a country code to save the file

## Examples

### First usage:

```bash
./querylist  --v4 ../../data/v4-top-1m.json --v6 ../../data/v6-top-1m.json --tls ../../data/tls-top-1m.json
```

This will generate `../../data/top-1m-tech-details.json`.

### Including Citizen Lab details:

```bash
./querylist --tech ../../data/top-1m-tech-details.json --cit-lab-global ../../data/global.csv --cit-lab-country ../../cn.csv -c CN
```

This will generate `../../data/CN-top-1m-ripe-ready.json`

The above two examples can be combined into a single command.

Note the results in the tech details and ripe ready json files will not be sorted any more.

To sort by Tranco rank you can run (from `data` directory):

```bash
cat CN-top-1m-ripe-ready.json | jq "sort_by(.tranco_rank)" > CN-top-1m-ripe-ready-sorted.json
```

# ZDNS

The ZDNS github page has more info in needed but basic usage to use ZDNS to get
the A records for the Tranco list of domains would be:

```bash
cat top-1m.csv | <path>/zdns A --alexa --output-file v4-top-1m.json
```

AAAA Records can be done similarly:

```bash
cat top-1m.csv | <path>/zdns AAAA --alexa --output-file v6-top-1m.json
```

This assumes the top-1m.csv is in the format provided by Tranco. You will want
significant bandwidth for this, on home networks there will be a lot of
timeouts.

# ZGrab

Likewise the ZGrab2 github page has more details, but usage is usually:

```bash
cat top-1m.csv | awk -F"," '{print $2}' | <path>/zgrab2 -o tls-top-1m.json tls
```