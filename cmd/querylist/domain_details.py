"""
Defines the DomainDetails class and its usage
"""

from collections.abc import Iterable
import json
from typing import Any, Dict

class DomainDetails:
    """
    Stores all the information about a domain in a nice wrapper, and prints
    as JSON
    """

    def __init__(self, details: Dict[str, Any]):
        self._domain = details.get("domain")
        self._tranco_rank = details.get("tranco_rank")
        self._has_v4 = details.get("has_v4")
        self._has_v6 = details.get("has_v6")
        self._on_citizen_lab_global_list = details.get("on_citizen_lab_global_list") or False
        self._citizen_lab_countries = details.get("citizen_lab_countries") or []
        self._citizen_lab_category = details.get("citizen_lab_category") or ""

    def get_domain(self) -> str:
        """
        Returns the domain associated with these details
        """
        return self._domain

    def set_domain(self, domain: str) -> None:
        """
        Changes the domain associated with these details
        """
        self._domain = domain

    def get_category(self) -> str:
        """
        Returns the category
        """
        return self._citizen_lab_category

    def set_category(self, category: str) -> None:
        """
        Changes the category
        """
        self._citizen_lab_category = category

    def add_citizen_lab_country(self, country_code: str) -> None:
        """
        Append a country code to _citizen_lab_countries
        """
        if country_code not in self._citizen_lab_countries:
            self._citizen_lab_countries.append(country_code)

    def update_citizen_lab_country(self, country_codes: Iterable) -> None:
        """
        Append multiple country codes in some iterable structure
        """
        for c_c in country_codes:
            self.add_citizen_lab_country(c_c)

    def get_citizen_lab_global(self) -> bool:
        """
        Returns whether the domain is on the global list
        """
        return self._on_citizen_lab_global_list

    def set_citizen_lab_global(self, on_global: bool) -> None:
        """
        Sets whether this domain is on the global list
        """
        self._on_citizen_lab_global_list = on_global

    def _to_dict(self) -> Dict[str, Any]:
        """
        Returns a nicely printed dict for turning into JSON
        """
        d = {}
        d["domain"] = self._domain
        d["tranco_rank"] = self._tranco_rank
        d["has_v4"] = self._has_v4
        d["has_v6"] = self._has_v6
        d["on_citizen_lab_global_list"] = self._on_citizen_lab_global_list
        d["citizen_lab_countries"] = self._citizen_lab_countries
        d["citizen_lab_category"] = self._citizen_lab_category
        return d

    def to_json(self):
        """
        Takes this structure and turns it back into a Dict and then JSON
        """
        return json.dumps(self._to_dict())

    @classmethod
    def base_instance(cls):
        """
        Creates a base instance of a DomainDetails
        """
        d_d = DomainDetails({})
        return d_d
