```terraform
import {
  to = aws_lambda_layer_version.test_layer
  id = "arn:aws:lambda:_REGION_:_ACCOUNT_ID_:layer:_LAYER_NAME_:_LAYER_VERSION_"
}
```

Using `pulumi import`, import Lambda Layers using `arn`. For example:

```console
% pulumi import \
    aws_lambda_layer_version.test_layer \
    arn:aws:lambda:_REGION_:_ACCOUNT_ID_:layer:_LAYER_NAME_:_LAYER_VERSION_
```
