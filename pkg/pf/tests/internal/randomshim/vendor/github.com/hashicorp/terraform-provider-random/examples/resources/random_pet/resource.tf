# The following example shows how to generate a unique pet name
# for an AWS EC2 instance that changes each time a new AMI id is
# selected.

resource "random_pet" "server" {
  keepers = {
    # Generate a new pet name each time we switch to a new AMI id
    ami_id = var.ami_id
  }
}

resource "aws_instance" "server" {
  tags = {
    Name = "web-server-${random_pet.server.id}"
  }

  # Read the AMI id "through" the random_pet resource to ensure that
  # both will change together.
  ami = random_pet.server.keepers.ami_id

  # ... (other aws_instance arguments) ...
}
