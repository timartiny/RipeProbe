#! /usr/bin/env python3

"""
This script will take the tech details JSON and append to it Citizen Lab data
"""


import argparse
import csv
import json
from os import listdir
from os.path import isdir, isfile, join
import re
from typing import Dict

from tldextract import extract

from domain_details import DomainDetails

def setup_args() -> argparse.Namespace:
    """
    Grabs command lines arguments
    """
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "tech_details",
        help="Path to the technical details from the top 1 million domains",
    )
    parser.add_argument(
        "citizen_lab_directory",
        help="Path to the directory containing the Citizen Lab lists"
    )
    parser.add_argument(
        "full_details",
        help="File to write all details to (in JSON)"
    )
    return parser.parse_args()

def setup_tech_details(path: str) -> Dict[str, DomainDetails]:
    """
    Takes the path to the tech details and reads them all in.

    Returns a dictionary indexed by domain of DomainDetails
    """
    ret = {}
    with open(path, 'r') as tech_file:
        for domain_str in tech_file.readlines():
            domain_details = DomainDetails(json.loads(domain_str))
            ret[domain_details.get_domain()] = domain_details

    return ret

def strip_url(full_url: str) -> str:
    """
    Takes a full url like https://www.google.com/ and returns the domain: google.com
    """
    sub, dom, tld = extract(full_url)
    url = dom + "." + tld
    if len(sub) > 0 and sub != "www":
        url = sub + "." + url
    return url

def update_tech_details(
        t_d: Dict[str, DomainDetails],
        path: str,
        country_code: str
    ) -> None:
    """
    Updates existing tech details with a given file's contents
    """
    with open(path, 'r', newline='') as csv_file:
        reader = csv.reader(csv_file)
        next(reader) # skip the first line (header line)
        for row in reader:
            url = strip_url(row[0])
            if t_d.get(url) is not None:
                domain_details = t_d[url]
                category = row[1]
                # this happens a lot actually, due to stripping everything after /
                # curr_category = domain_details.get_category()
                # if len(curr_category) > 0 and category != curr_category:
                    # print(
                    #     f"Existing category for {domain_details.get_domain()}"+
                    #     f" ({curr_category}) does not match category from"+
                    #     f" {path} ({category})",
                    # )
                    # print("Changing to use new category")
                domain_details.set_category(category)
                if country_code == "GLOBAL":
                    domain_details.set_citizen_lab_global(True)
                else:
                    domain_details.add_citizen_lab_country(country_code)

def fill_in_citizen_lab_data(
        t_d: Dict[str, DomainDetails],
        cit_lab_dir: str
    ) -> Dict[str, DomainDetails]:
    """
    This will read each of the lists from Citizen Lab and fill in for each domain
    """
    assert isdir(cit_lab_dir)
    onlyfiles = [f for f in listdir(cit_lab_dir) if isfile(join(cit_lab_dir, f))]
    for file_name in onlyfiles:
        if file_name == "global.csv" or re.match('^[a-z]{2}.csv', file_name) is not None:
            country_code = file_name.split(".")[0].upper()
            update_tech_details(t_d, join(cit_lab_dir, file_name), country_code)

    return

def save_tech_dict(t_d: Dict[str, DomainDetails], path: str) -> None:
    """
    Opens the file and writes the tech details, now with Citizen Lab data, to
    that file, each line is JSON, but not the file overall.
    """
    with open(path, 'w') as write_file:
        for key in t_d:
            write_file.write(t_d[key].to_json() + "\n")


if __name__ == "__main__":
    args = setup_args()
    print(f"Reading in Tech Details from {args.tech_details}")
    tech_dict = setup_tech_details(args.tech_details)
    print(tech_dict["google.com"].to_json())
    print(f"Filling in Citizen Lab data from {args.citizen_lab_directory}")
    fill_in_citizen_lab_data(tech_dict, args.citizen_lab_directory)
    print(tech_dict["google.com"].to_json())
    print(f"Saving data to {args.full_details}")
    save_tech_dict(tech_dict, args.full_details)
