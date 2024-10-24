resource "random_shuffle" "az" {
  input        = ["us-west-1a", "us-west-1c", "us-west-1d", "us-west-1e"]
  result_count = 2
}

resource "aws_elb" "example" {
  # Place the ELB in any two of the given availability zones, selected
  # at random.
  availability_zones = random_shuffle.az.result

  # ... and other aws_elb arguments ...
}
