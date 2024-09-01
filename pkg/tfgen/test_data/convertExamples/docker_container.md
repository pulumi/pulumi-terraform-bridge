Import is supported using the following syntax by providing the `id`:

```shell
#!/bin/bash
terraform import docker_container.foo id
```

### Example

Assuming you created a `container` as follows

```shell
#!/bin/bash
docker run --name foo -p8080:80 -d nginx 
# prints the container ID 
9a550c0f0163d39d77222d3efd58701b625d47676c25c686c95b5b92d1cba6fd
```

you provide the definition for the resource as follows

```terraform
resource "docker_container" "foo" {
  name  = "foo"
  image = "nginx"

  ports {
    internal = "80"
    external = "8080"
  }
}
```

then the import command is as follows

```shell
#!/bin/bash
terraform import docker_container.foo 9a550c0f0163d39d77222d3efd58701b625d47676c25c686c95b5b92d1cba6fd
```