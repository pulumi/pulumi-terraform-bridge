With this resource you can manage custom HTML for the Login, Reset Password, Multi-Factor Authentication and Error pages.

## Example Usage

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as auth0 from "@pulumi/auth0";

const myPages = new auth0.Pages("myPages", {
    changePassword: {
        enabled: true,
        html: "<html><body>My Custom Reset Password Page</body></html>",
    },
    error: {
        html: "<html><body>My Custom Error Page</body></html>",
        showLogLink: true,
        url: "https://example.com",
    },
    guardianMfa: {
        enabled: true,
        html: "<html><body>My Custom MFA Page</body></html>",
    },
    login: {
        enabled: true,
        html: "<html><body>My Custom Login Page</body></html>",
    },
});
```
```python
import pulumi
import pulumi_auth0 as auth0

my_pages = auth0.Pages("myPages",
    change_password={
        "enabled": True,
        "html": "<html><body>My Custom Reset Password Page</body></html>",
    },
    error={
        "html": "<html><body>My Custom Error Page</body></html>",
        "show_log_link": True,
        "url": "https://example.com",
    },
    guardian_mfa={
        "enabled": True,
        "html": "<html><body>My Custom MFA Page</body></html>",
    },
    login={
        "enabled": True,
        "html": "<html><body>My Custom Login Page</body></html>",
    })
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Auth0 = Pulumi.Auth0;

return await Deployment.RunAsync(() => 
{
    var myPages = new Auth0.Pages("myPages", new()
    {
        ChangePassword = new Auth0.Inputs.PagesChangePasswordArgs
        {
            Enabled = true,
            Html = "<html><body>My Custom Reset Password Page</body></html>",
        },
        Error = new Auth0.Inputs.PagesErrorArgs
        {
            Html = "<html><body>My Custom Error Page</body></html>",
            ShowLogLink = true,
            Url = "https://example.com",
        },
        GuardianMfa = new Auth0.Inputs.PagesGuardianMfaArgs
        {
            Enabled = true,
            Html = "<html><body>My Custom MFA Page</body></html>",
        },
        Login = new Auth0.Inputs.PagesLoginArgs
        {
            Enabled = true,
            Html = "<html><body>My Custom Login Page</body></html>",
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-auth0/sdk/v3/go/auth0"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := auth0.NewPages(ctx, "myPages", &auth0.PagesArgs{
			ChangePassword: &auth0.PagesChangePasswordArgs{
				Enabled: pulumi.Bool(true),
				Html:    pulumi.String("<html><body>My Custom Reset Password Page</body></html>"),
			},
			Error: &auth0.PagesErrorArgs{
				Html:        pulumi.String("<html><body>My Custom Error Page</body></html>"),
				ShowLogLink: pulumi.Bool(true),
				Url:         pulumi.String("https://example.com"),
			},
			GuardianMfa: &auth0.PagesGuardianMfaArgs{
				Enabled: pulumi.Bool(true),
				Html:    pulumi.String("<html><body>My Custom MFA Page</body></html>"),
			},
			Login: &auth0.PagesLoginArgs{
				Enabled: pulumi.Bool(true),
				Html:    pulumi.String("<html><body>My Custom Login Page</body></html>"),
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
import com.pulumi.auth0.Pages;
import com.pulumi.auth0.PagesArgs;
import com.pulumi.auth0.inputs.PagesChangePasswordArgs;
import com.pulumi.auth0.inputs.PagesErrorArgs;
import com.pulumi.auth0.inputs.PagesGuardianMfaArgs;
import com.pulumi.auth0.inputs.PagesLoginArgs;
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
        var myPages = new Pages("myPages", PagesArgs.builder()
            .changePassword(PagesChangePasswordArgs.builder()
                .enabled(true)
                .html("<html><body>My Custom Reset Password Page</body></html>")
                .build())
            .error(PagesErrorArgs.builder()
                .html("<html><body>My Custom Error Page</body></html>")
                .showLogLink(true)
                .url("https://example.com")
                .build())
            .guardianMfa(PagesGuardianMfaArgs.builder()
                .enabled(true)
                .html("<html><body>My Custom MFA Page</body></html>")
                .build())
            .login(PagesLoginArgs.builder()
                .enabled(true)
                .html("<html><body>My Custom Login Page</body></html>")
                .build())
            .build());

    }
}
```
```yaml
resources:
  myPages:
    type: auth0:Pages
    properties:
      changePassword:
        enabled: true
        html: <html><body>My Custom Reset Password Page</body></html>
      error:
        html: <html><body>My Custom Error Page</body></html>
        showLogLink: true
        url: https://example.com
      guardianMfa:
        enabled: true
        html: <html><body>My Custom MFA Page</body></html>
      login:
        enabled: true
        html: <html><body>My Custom Login Page</body></html>
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

