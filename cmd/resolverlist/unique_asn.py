#! /usr/bin/env python3

"""
Looks up provided resolver lists in

<ip_addr> <source>

format and prints those in unique ASNs
"""

from collections import defaultdict
import argparse
from typing import List
import geoip2
import geoip2.database

def setup_args() -> argparse.Namespace:
    """
    Grabs command lines arguments
    """
    parser = argparse.ArgumentParser()
    parser.add_argument("asnDB", help="Path to the geoip2 ASN database")
    parser.add_argument("resolvers", help="Path to the resolver list file")
    return parser.parse_args()

def lookup_resolvers(database: str, resolvers: str) -> List[str]:
    """
    Uses the provided database to look up resolver IPs for unique ASNs
    """
    ret = []
    asn_dict = defaultdict(bool)

    with geoip2.database.Reader(fileish=database) as reader:
        resolver_file = open(file=resolvers, mode='r')
        lines = resolver_file.readlines()
        resolver_file.close()
        for line in lines:
            ip_addr = line.split(" ")[0]
            try:
                response = reader.asn(ip_addr)
                if not asn_dict[response.autonomous_system_number]:
                    ret.append(line)
                    asn_dict[response.autonomous_system_number] = True
            except geoip2.errors.AddressNotFoundError:
                ret.append(line)
                continue
    return ret

if __name__ == "__main__":
    args = setup_args()
    shortened_resolvers = lookup_resolvers(
        database=args.asnDB, resolvers=args.resolvers
    )
    for resolver_line in shortened_resolvers:
        print(resolver_line.strip())
