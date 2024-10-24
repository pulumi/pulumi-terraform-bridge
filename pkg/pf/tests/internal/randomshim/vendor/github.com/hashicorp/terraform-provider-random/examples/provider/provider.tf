resource "random_id" "server" {
  keepers = {
    # Generate a new id each time we switch to a new AMI id
    ami_id = var.ami_id
  }

  byte_length = 8
}

resource "aws_instance" "server" {
  tags = {
    Name = "web-server ${random_id.server.hex}"
  }

  # Read the AMI id "through" the random_id resource to ensure that
  # both will change together.
  ami = random_id.server.keepers.ami_id

  # ... (other aws_instance arguments) ...
}
