# Parse Whiteboard Experiment

The `parse_whiteboard_experiment.py` script will read in the raw RIPE Atlas
results and save simplified results as well as all the unique pairings of (IP,
Domain) that can later be passed to zgrab to do look ups.

```
usage: parse_whiteboard_experiment.py [-h]
                                      measurement_file simplified_file
                                      ip_dom_map_file

positional arguments:
  measurement_file  Path to the file containing the list of measurement IDs
  simplified_file   Path to write the simplified JSON output to
  ip_dom_map_file   File to write all the unique (IP, Domain) pairings

optional arguments:
  -h, --help        show this help message and exit
```

Usage will look like:

`./parse_whiteboard_experiment.py data/Whiteboard-Ids-CN-2021-09-09::15:30
data/simplified_results_CN_2021_09_09::15:30.json
data/ip_dom_pairs_CN_2021_09_09::15:30`