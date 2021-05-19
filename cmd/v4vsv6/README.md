# V4 vs V6

This code should probably be replaced with calls to ZGrab using the TLS module.

But as is, it works due to the low number of connections it needs to make.

It will read the Whiteboard results file and attempt to connect to each IP that was resolved and attempt to check it's cert for validity based on the expected domain.

From this it will identify which IPs provided were valid or invalid.

Usage:
```bash
./v4vsv6 -r ../../data/<meas_id1>-<meas_id2>/Whiteboard_results<meas_id1>-<meas_id2>.json -u "<string of comma separated domains to considered 'unblocked'>"
```