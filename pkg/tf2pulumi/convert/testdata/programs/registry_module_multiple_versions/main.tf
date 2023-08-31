#if EXPERIMENTAL
module "dir-1_0_0" {
  source  = "hashicorp/dir/template"
  version = "1.0.0"

  base_dir = "./src"
  template_vars = {
    vpc_id = "vpc-abc123"
  }
}

module "dir-1_0_2" {
  source  = "hashicorp/dir/template"
  version = "1.0.2"

  base_dir = "./otherSrc"
  template_vars = {
    some_flag = true
  }
}
#else
// Modules are not supported
#endif