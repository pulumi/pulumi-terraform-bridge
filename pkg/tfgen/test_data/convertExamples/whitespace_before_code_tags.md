With this resource you can manage custom HTML for the Login, Reset Password, Multi-Factor Authentication and Error pages.

## Example Usage

First example, should translate 
  ```hcl
resource "auth0_pages" "my_pages" {
  login {
    enabled = true
    html    = "<html><body>My Custom Login Page</body></html>"
  }

  change_password {
    enabled = true
    html    = "<html><body>My Custom Reset Password Page</body></html>"
  }

  guardian_mfa {
    enabled = true
    html    = "<html><body>My Custom MFA Page</body></html>"
  }

  error {
    show_log_link = true
    html          = "<html><body>My Custom Error Page</body></html>"
    url           = "https://example.com"
  }
}
  ```

Second example, with a tab; should still translate

    ```hcl
    resource "auth0_pages" "my_pages" {
      login {
        enabled = true
        html    = "<html><body>My Custom Login Page</body></html>"
      }
    
      change_password {
        enabled = true
        html    = "<html><body>My Custom Reset Password Page</body></html>"
      }
    
      guardian_mfa {
        enabled = true
        html    = "<html><body>My Custom MFA Page</body></html>"
      }
    
      error {
        show_log_link = true
        html          = "<html><body>My Custom Error Page</body></html>"
        url           = "https://example.com"
      }
    }
    ```

Third example, no whitespace
```hcl
resource "auth0_pages" "my_pages" {
  login {
    enabled = true
    html    = "<html><body>My Custom Login Page</body></html>"
  }

  change_password {
    enabled = true
    html    = "<html><body>My Custom Reset Password Page</body></html>"
  }

  guardian_mfa {
    enabled = true
    html    = "<html><body>My Custom MFA Page</body></html>"
  }

  error {
    show_log_link = true
    html          = "<html><body>My Custom Error Page</body></html>"
    url           = "https://example.com"
  }
}
```

## Import

As this is not a resource identifiable by an ID within the Auth0 Management API,

pages can be imported using a random string.

#

We recommend [Version 4 UUID](https://www.uuidgenerator.net/version4)

#

Example:

```sh
$ pulumi import auth0:index/pages:Pages my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"
```

