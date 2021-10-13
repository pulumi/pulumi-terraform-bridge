resource "aws_vpc_endpoint" "private_s3" {
  vpc_id       = aws_vpc.foo.id
  service_name = "com.amazonaws.us-west-2.s3"
}

data "aws_prefix_list" "private_s3" {
  prefix_list_id = aws_vpc_endpoint.private_s3.prefix_list_id
}

resource "aws_network_acl" "bar" {
  vpc_id = aws_vpc.foo.id
}

resource "aws_network_acl_rule" "private_s3" {
  network_acl_id = aws_network_acl.bar.id
  rule_number    = 200
  egress         = false
  protocol       = "tcp"
  rule_action    = "allow"
  cidr_block     = data.aws_prefix_list.private_s3.cidr_blocks[0]
  from_port      = 443
  to_port        = 443
}
