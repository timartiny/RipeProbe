# Whiteboard Results

This script will take results of RIPE measurements that have already been
fetched and put the pertinent bits into a JSON struct for use in future steps.

Usage:
```bash
./whiteboardresults -m ../../data/Whiteboard-Ids-<country_code>-<timestamp> -r ../../data/<country_code>_resolvers_ips.dat
```

This will create the file in the `data/<measurement_id>-<measurement_id>/`
directory (based on the measurements in the experiment) called
`Whiteboard_results<measurement_id>-<measurement_id>.json`.