#! /usr/bin/env python3

"""
This module will take the data from Zdns and Zgrab and group all the queries
into JSON to track the technical data: whether a domain has a v4 address, a v6
address, supports tls on those v4 addresses, and on the v6 addresses.
"""

import argparse
import ipaddress
import json
from typing import Dict

from domain_details import DomainDetails

def setup_args() -> argparse.Namespace:
    """
    Grabs command lines arguments
    """
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "v4_dns",
        help="Path to the zdns results for v4 lookups",
    )
    parser.add_argument(
        "v6_dns",
        help="Path to the zdns results for v6 lookups",
    )
    # parser.add_argument(
    #     "v4_tls",
    #     help="Path to the zgrab results for v4 TLS banner grabs",
    # )
    # parser.add_argument(
    #     "v6_tls",
    #     help="Path to the zgrab results for v6 TLS banner grabs",
    # )
    # parser.add_argument(
    #     "out_file",
    #     help="File to write all the tech details to (in JSON)"
    # )
    return parser.parse_args()

def add_dns_results(path: str, tech_dict: Dict[str, DomainDetails]):
    """
    This will read lines from path and fill in the tech dictionary with results
    """
    with open(path, 'r') as dns_file:
        for dns_result_string in dns_file.readlines():
            dns_result_dict = json.loads(dns_result_string)
            domain = dns_result_dict.get("name")
            if tech_dict.get(domain) is not None:
                domain_details = tech_dict[domain]
            else:
                domain_details = DomainDetails(
                    {
                        "domain": domain,
                        "tranco_rank": dns_result_dict.get("alexa_rank")
                    }
                )
            dns_data_dict = dns_result_dict.get("data")
            if dns_data_dict is not None:
                for dns_answer_dict in dns_data_dict.get("answers", []):
                    dns_type = dns_answer_dict.get("type")
                    if (dns_type != "A") and (dns_type != "AAAA"):
                        continue
                    dns_answer = dns_answer_dict.get("answer")
                    try:
                        ip_address = ipaddress.ip_address(dns_answer)
                    except ValueError:
                        print(
                            f"Got an invalid IP address ({dns_answer}) in DNS "
                                f"answer: {dns_answer_dict}"
                        )
                        continue
                    if ip_address.version == 4 and dns_type == "A":
                        domain_details.set_has_v4(True)
                    elif ip_address.version == 6 and dns_type == "AAAA":
                        domain_details.set_has_v6(True)
                    else:
                        print(
                            f"Unusual mismatch of IP version ({ip_address}, "
                                f"version: {ip_address.version}) and DNS type: "
                                f"{dns_type} for answer: {dns_answer_dict}"
                        )
                        continue
            tech_dict[domain_details.get_domain()] = domain_details

if __name__ == "__main__":
    tech_details_dictionary: Dict[str, DomainDetails] = {}
    args = setup_args()
    print(f"Reading in v4 DNS query results from {args.v4_dns}")
    add_dns_results(args.v4_dns, tech_details_dictionary)
    print(f"google tech details so far: {tech_details_dictionary['google.com']}")
    print(f"Reading in v6 DNS query results from {args.v6_dns}")
    add_dns_results(args.v6_dns, tech_details_dictionary)
    print(f"google tech details so far: {tech_details_dictionary['google.com']}")
    tls_details_dictionary: Dict[str, Dict[str, bool]] = {}
