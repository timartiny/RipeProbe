#! /usr/bin/env python3

"""
This module will take JSON results from the Whiteboard Experiment and save more usable JSON results
"""

import argparse
import json
from collections import defaultdict
from typing import Any, Dict, List
from ripe.atlas.sagan import DnsResult

ip_dom_map = defaultdict(bool)

def setup_args() -> argparse.Namespace:
    """
    Grabs command lines arguments
    """
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "measurement_file", 
        help="Path to the file containing the list of measurement IDs",
    )
    parser.add_argument(
        "simplified_file",
        help="Path to write the simplified JSON output to"
    )
    parser.add_argument(
        "ip_dom_map_file",
        help="File to write all the unique (IP, Domain) pairings"
    )
    return parser.parse_args()

def get_measurement_ids(measurement_file: str) -> List[int]:
    """
    Takes the file containing the list of file IDs and saves them as a list of 
    ints
    """
    ret = []
    with open(measurement_file) as mf:
        for mid in mf.readlines():
            ret.append(int(mid))

    return ret

def simplify_single_result(probe_result: DnsResult, record_type: str, domain: str, fp):
    """
    This will take a single result from a probe and simplify it
    """
    simplified_result = defaultdict(str)
    simplified_result["probe_id"] = probe_result.probe_id
    simplified_result["had_error"] = probe_result.is_error
    simplified_result["record_type"] = record_type
    simplified_result["domain"] = domain
    if probe_result.is_error:
        # in this case we can't get out whether A or AAAA was requested :(
        simplified_result["error"] = probe_result.error_message
        simplified_result["resolver"] = probe_result.raw_data["dst_addr"]
    else:
        for ripe_response in probe_result.responses:
            simplified_result["resolver"] = ripe_response.destination_address
            dns_message = ripe_response.abuf
            if len(dns_message.questions) > 1:
                print("Got more than one question, new territory")
            answers = []
            for answer in dns_message.answers:
                if simplified_result["domain"] != answer.name[:-1]:
                    print("MISMATCH QUESTION AND ANSWER NAME")
                    print(
                        f"question name: {simplified_result['domain']} answer "+
                            f"name: {answer.name[:-1]}",
                    )
                answers.append(answer.address)
                ip_dom_str = f"{answer.address}, {answer.name[:-1]}"
                ip_dom_map[ip_dom_str] = True
            simplified_result["answers"] = answers

    fp.write(json.dumps(simplified_result))


def simplify_file_results(file_results: List[Dict[str, Any]], fp):
    """
    This will take a singular file's results and simplify it
    """
    # Annoyingly if there is an error you can't determine whether you requested
    # A or AAAA records, so we loop through to find out first, then print results

    for probe_result in file_results:
        dns_result = DnsResult(probe_result)
        if dns_result.is_error:
            continue
        record_type = dns_result.responses[0].abuf.questions[0].type
        domain = dns_result.responses[0].abuf.questions[0].name[:-1]
    for ind, probe_result in enumerate(file_results):
        simplify_single_result(DnsResult(probe_result), record_type, domain, fp)
        if ind != len(file_results) - 1:
            fp.write(",")

def simplify_all_results(ids: List[int], folder: str, simp_file: str):
    """
    Will read in the various results from each file and simplify them for saving
    """
    with open(simp_file, 'w') as file_pointer:
        file_pointer.write("[")
        for meas_ind, measurement_id in enumerate(ids):
            with open(f"{folder}/{measurement_id}_results.json", 'r') as reader:
                lines = reader.readlines()
            for ind, result in enumerate(lines):
                simplify_file_results(json.loads(result), file_pointer)
                if ind != len(lines) - 1:
                    file_pointer.write(",")
            if meas_ind != len(ids) - 1:
                file_pointer.write(",")
        file_pointer.write("]")

def write_ip_dom_map(path: str):
    """
    Write all the (IP, Domain) pairings
    """
    with open(path, 'w') as fp:
        for pairing in ip_dom_map.keys():
            fp.write(pairing + "\n")

if __name__ == "__main__":
    DATA_PREFIX = "data"
    args = setup_args()
    measurement_ids = get_measurement_ids(args.measurement_file)
    results_folder = f"{DATA_PREFIX}/{measurement_ids[0]}-{measurement_ids[-1]}/"
    simplify_all_results(measurement_ids, results_folder, args.simplified_file)
    write_ip_dom_map(args.ip_dom_map_file)
