import argparse
import csv
from dataclasses import dataclass, field
import enum
import json
from typing import Any


class Element(enum.Enum):
    RESOURCE_ELEMENT = "resource_element"
    ATTRIBUTE_ELEMENT = "attribute_element"


@dataclass
class Results:
    provider_name: str
    resources: list[dict[str, Any]] = field(default_factory=list)
    attributes: list[dict[str, Any]] = field(default_factory=list)

    def add_resource(self, name: str, resource: dict[str, Any]):
        resource["Provider"] = self.provider_name
        resource["Name"] = name
        self.resources.append(resource)

    def add_attribute(self, name: str, attribute: dict[str, Any]):
        attribute["Provider"] = self.provider_name
        attribute["Name"] = name
        self.attributes.append(attribute)

    def parse_res_or_attr(self, elem_schema: dict[str, Any], name: str) -> Element:
        if "Schema" in elem_schema:
            # this is a resource
            self.add_resource(name, self.parse_resource(elem_schema, name))
            return Element.RESOURCE_ELEMENT
        self.add_attribute(name, self.parse_attribute(elem_schema, name))
        return Element.ATTRIBUTE_ELEMENT

    def parse_attribute(
        self, attribute_schema: dict[str, Any], name: str
    ) -> dict[str, Any]:
        res: dict[str, Any] = {}
        for key, value in attribute_schema.items():
            if key == "Elem":
                continue
            res[key] = value

        elem = attribute_schema.get("Elem")
        if elem:
            elem_name = f"{name}.elem"
            key = self.parse_res_or_attr(elem, elem_name)
            res[key.value] = elem_name

        return res

    def parse_resource(
        self, resource_schema: dict[str, Any], name: str
    ) -> dict[str, Any]:
        res: dict[str, Any] = {}
        for key, value in resource_schema.items():
            if key == "Schema":
                continue
            res[key] = value

        schema_list: list[str] = []
        attributes: dict[str, Any] = resource_schema.get("Schema", {})
        for attribute_name, attribute_schema in attributes.items():
            full_name = f"{name}.{attribute_name}"
            self.add_attribute(
                full_name, self.parse_attribute(attribute_schema, full_name)
            )
            schema_list.append(full_name)
        res["Schema"] = schema_list
        return res


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tf-schema-file", type=str, required=True)
    ap.add_argument("--provider-name", type=str, required=True)
    args = ap.parse_args()
    schema_file = args.tf_schema_file
    provider_name = args.provider_name

    with open(schema_file, encoding="utf-8") as f:
        provider_schema = json.load(f)

    res = Results(provider_name=provider_name)

    for resource_name, resource_schema in provider_schema["ResourcesMap"].items():
        res.add_resource(
            resource_name, res.parse_resource(resource_schema, resource_name)
        )

    res_keys = set(res.resources[0].keys())
    res_keys.update([Element.ATTRIBUTE_ELEMENT.value, Element.RESOURCE_ELEMENT.value])

    attr_keys = set(res.attributes[0].keys())
    attr_keys.update([Element.ATTRIBUTE_ELEMENT.value, Element.RESOURCE_ELEMENT.value])

    with open(f"{provider_name}_resources.csv", "w", encoding="utf-8") as csvfile:
        writer = csv.DictWriter(csvfile, fieldnames=res_keys)
        writer.writeheader()
        writer.writerows(res.resources)

    with open(f"{provider_name}_attributes.csv", "w", encoding="utf-8") as csvfile:
        writer = csv.DictWriter(csvfile, fieldnames=attr_keys)
        writer.writeheader()
        writer.writerows(res.attributes)


if __name__ == "__main__":
    main()
