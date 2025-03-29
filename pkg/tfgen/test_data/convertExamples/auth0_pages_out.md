With this resource you can manage custom HTML for the Login, Reset Password, Multi-Factor Authentication and Error pages.

## Example Usage

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as auth0 from "@pulumi/auth0";

const myPages = new auth0.index.Auth0_pages("myPages", {
    login: [{
        enabled: true,
        html: "<html><body>My Custom Login Page</body></html>",
    }],
    changePassword: [{
        enabled: true,
        html: "<html><body>My Custom Reset Password Page</body></html>",
    }],
    guardianMfa: [{
        enabled: true,
        html: "<html><body>My Custom MFA Page</body></html>",
    }],
    error: [{
        showLogLink: true,
        html: "<html><body>My Custom Error Page</body></html>",
        url: "https://example.com",
    }],
});
```
```python
import pulumi
import pulumi_auth0 as auth0

my_pages = auth0.index.Auth0_pages("myPages",
    login=[{
        enabled: True,
        html: <html><body>My Custom Login Page</body></html>,
    }],
    change_password=[{
        enabled: True,
        html: <html><body>My Custom Reset Password Page</body></html>,
    }],
    guardian_mfa=[{
        enabled: True,
        html: <html><body>My Custom MFA Page</body></html>,
    }],
    error=[{
        showLogLink: True,
        html: <html><body>My Custom Error Page</body></html>,
        url: https://example.com,
    }])
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Auth0 = Pulumi.Auth0;

return await Deployment.RunAsync(() => 
{
    var myPages = new Auth0.Index.Auth0_pages("myPages", new()
    {
        Login = new[]
        {
            
            {
                { "enabled", true },
                { "html", "<html><body>My Custom Login Page</body></html>" },
            },
        },
        ChangePassword = new[]
        {
            
            {
                { "enabled", true },
                { "html", "<html><body>My Custom Reset Password Page</body></html>" },
            },
        },
        GuardianMfa = new[]
        {
            
            {
                { "enabled", true },
                { "html", "<html><body>My Custom MFA Page</body></html>" },
            },
        },
        Error = new[]
        {
            
            {
                { "showLogLink", true },
                { "html", "<html><body>My Custom Error Page</body></html>" },
                { "url", "https://example.com" },
            },
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-auth0/sdk/go/auth0"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := auth0.NewAuth0_pages(ctx, "myPages", &auth0.Auth0_pagesArgs{
			Login: []map[string]interface{}{
				map[string]interface{}{
					"enabled": true,
					"html":    "<html><body>My Custom Login Page</body></html>",
				},
			},
			ChangePassword: []map[string]interface{}{
				map[string]interface{}{
					"enabled": true,
					"html":    "<html><body>My Custom Reset Password Page</body></html>",
				},
			},
			GuardianMfa: []map[string]interface{}{
				map[string]interface{}{
					"enabled": true,
					"html":    "<html><body>My Custom MFA Page</body></html>",
				},
			},
			Error: []map[string]interface{}{
				map[string]interface{}{
					"showLogLink": true,
					"html":        "<html><body>My Custom Error Page</body></html>",
					"url":         "https://example.com",
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.auth0.auth0_pages;
import com.pulumi.auth0.auth0_pagesArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var myPages = new Auth0_pages("myPages", Auth0_pagesArgs.builder()
            .login(List.of(Map.ofEntries(
                Map.entry("enabled", true),
                Map.entry("html", "<html><body>My Custom Login Page</body></html>")
            )))
            .changePassword(List.of(Map.ofEntries(
                Map.entry("enabled", true),
                Map.entry("html", "<html><body>My Custom Reset Password Page</body></html>")
            )))
            .guardianMfa(List.of(Map.ofEntries(
                Map.entry("enabled", true),
                Map.entry("html", "<html><body>My Custom MFA Page</body></html>")
            )))
            .error(List.of(Map.ofEntries(
                Map.entry("showLogLink", true),
                Map.entry("html", "<html><body>My Custom Error Page</body></html>"),
                Map.entry("url", "https://example.com")
            )))
            .build());

    }
}
```
```yaml
resources:
  myPages:
    type: auth0:auth0_pages
    properties:
      login:
        - enabled: true
          html: <html><body>My Custom Login Page</body></html>
      changePassword:
        - enabled: true
          html: <html><body>My Custom Reset Password Page</body></html>
      guardianMfa:
        - enabled: true
          html: <html><body>My Custom MFA Page</body></html>
      error:
        - showLogLink: true
          html: <html><body>My Custom Error Page</body></html>
          url: https://example.com
```
<!--End PulumiCodeChooser -->

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

