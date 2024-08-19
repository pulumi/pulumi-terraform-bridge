# Example auto-fill

This is an opt-in feature that helps bridged provider maintainers improve the quality of the generated docs.

## The undeclared resource problem

The problem targeted by auto-fill is the problem of undeclared resources in the example code.

Bridged providers translate code examples from TF/HCL to Pulumi-supported languages. A frequent situation with the
source HCL examples is that they contain dangling references, that is running `terraform plan` would result in errors of
the form:

    Reference to undeclared resource

This problem by default gets preserved by the bridge, resulting in an example that fails to `pulumi up`, and sometimes
does not even compile.

## Authoring reference resource configurations

To activate auto-fill, the provider maintainer needs to first author reference resource configurations that will be used
in place of undeclared resources. This requires creating a folder, such as `./docs/auto-fill` with the following
structure:

```shell
docs/
  auto-fill/
    aws_acm_certificate.tf
    aws_route53_zone.tf
    ...
```

Each file in this folder is named after a resource type and contains a TF declaration that supplies a reference
configuration for this resource. It is required that the instance is named `"example"`, as in:

```terraform
resource "aws_acm_certificate" "example" {
  domain_name       = "example.com"
  validation_method = "DNS"
}
```

## Configuring auto-fill

Currently auto-fill can be activated by setting an environment variable:

```shell
export PULUMI_CONVERT_AUTOFILL_DIR=$PWD/docs/auto-fill
```

## Concrete illustration of auto-fill in action

Consider a real usage example from the [AWS provider](https://github.com/pulumi/pulumi-aws) for the `aws_route53_record`
resource that references an `aws_route53_zone` and `aws_acm_certificate` without defining these resources:

```terraform
resource "aws_route53_record" "example" {
      for_each = {
        for dvo in aws_acm_certificate.example.domain_validation_options : dvo.domain_name => {
          name   = dvo.resource_record_name
          record = dvo.resource_record_value
          type   = dvo.resource_record_type
        }
      }

      allow_overwrite = true
      name            = each.value.name
      records         = [each.value.record]
      ttl             = 60
      type            = each.value.type
      zone_id         = aws_route53_zone.example.zone_id
}
```

Add `docs/auto-fill/aws_route53_record.tf` and `docs/auto-fill/aws_acm_certificate`. Note that the file name is based on
the TF resource type:

```terraform

# docs/auto-fill/aws_acm_certificate.tf
resource "aws_acm_certificate" "example" {
  domain_name       = "example.com"
  validation_method = "DNS"
}

# docs/auto-fill/aws_route54_zone.tf
resource "aws_route53_zone" "example1" {
  name = "example.com"
}
```

Make sure each instance is called `"example"`.

Configure the build to locate the auto-fill examples:

```shell
export PULUMI_CONVERT_AUTOFILL_DIR=$PWD/docs/auto-fill
```

Rebuild the provider (`make tfgen`).

Observe that a Route53 zone has been added to the Route53 record examples. The translation proceeds as if the original
source had this extended form:


```terraform
resource "aws_route53_record" "example" {
      for_each = {
        for dvo in aws_acm_certificate.example.domain_validation_options : dvo.domain_name => {
          name   = dvo.resource_record_name
          record = dvo.resource_record_value
          type   = dvo.resource_record_type
        }
      }

      allow_overwrite = true
      name            = each.value.name
      records         = [each.value.record]
      ttl             = 60
      type            = each.value.type
      zone_id         = aws_route53_zone.example.zone_id
}

resource "aws_acm_certificate" "example" {
  domain_name       = "example.com"
  validation_method = "DNS"
}

resource "aws_route53_zone" "example" {
  name = "example.com"
}
```
