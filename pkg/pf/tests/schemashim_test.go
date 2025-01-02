// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridgetests

// Test how various PF-based schemata translate to the shim.Schema layer. Excerpts of the resulting Pulumi Package
// Schema are included for reasoning convenience.
//
// References:
//
// https://developer.hashicorp.com/terraform/plugin/framework/handling-data/attributes
// https://developer.hashicorp.com/terraform/plugin/framework/handling-data/blocks

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

func TestShimBoolAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"bool_attr": schema.BoolAttribute{Optional: true},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "bool_attr": {
        "optional": true,
        "type": 1
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "boolAttr": {
        "type": "boolean"
      }
    },
    "inputProperties": {
      "boolAttr": {
        "type": "boolean"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "boolAttr": {
          "type": "boolean"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimStringAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"str_attr": schema.StringAttribute{Optional: true},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "str_attr": {
        "optional": true,
        "type": 4
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "strAttr": {
        "type": "string"
      }
    },
    "inputProperties": {
      "strAttr": {
        "type": "string"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "strAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimNumberAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"num_attr": schema.NumberAttribute{Optional: true},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "num_attr": {
        "optional": true,
        "type": 3
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "numAttr": {
        "type": "number"
      }
    },
    "inputProperties": {
      "numAttr": {
        "type": "number"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "numAttr": {
          "type": "number"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimListOfStringAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"list_attr": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "list_attr": {
        "element": {
          "schema": {
            "type": 4
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "listAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "inputProperties": {
      "listAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "listAttrs": {
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimMapOfStringAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"map_attr": schema.MapAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "map_attr": {
        "element": {
          "schema": {
            "type": 4
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "mapAttr": {
        "type": "object",
        "additionalProperties": {
          "type": "string"
        }
      }
    },
    "inputProperties": {
      "mapAttr": {
        "type": "object",
        "additionalProperties": {
          "type": "string"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "mapAttr": {
          "type": "object",
          "additionalProperties": {
            "type": "string"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimSetOfStringAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"set_attr": schema.SetAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "set_attr": {
        "element": {
          "schema": {
            "type": 4
          }
        },
        "optional": true,
        "type": 7
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "setAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "inputProperties": {
      "setAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "setAttrs": {
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimListNestedAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"list_nested_attr": schema.ListNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"x": schema.StringAttribute{Optional: true},
						},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "list_nested_attr": {
        "element": {
          "schema": {
            "element": {
              "resource": {
                "x": {
                  "optional": true,
                  "type": 4
                }
              }
            },
            "type": 6
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "listNestedAttrs": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1ListNestedAttr:R1ListNestedAttr"
        }
      }
    },
    "inputProperties": {
      "listNestedAttrs": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1ListNestedAttr:R1ListNestedAttr"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "listNestedAttrs": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/R1ListNestedAttr:R1ListNestedAttr"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1ListNestedAttr:R1ListNestedAttr": {
      "properties": {
        "x": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimSetNestedAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"set_nested_attr": schema.SetNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"x": schema.StringAttribute{Optional: true},
						},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "set_nested_attr": {
        "element": {
          "schema": {
            "element": {
              "resource": {
                "x": {
                  "optional": true,
                  "type": 4
                }
              }
            },
            "type": 6
          }
        },
        "optional": true,
        "type": 7
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "setNestedAttrs": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1SetNestedAttr:R1SetNestedAttr"
        }
      }
    },
    "inputProperties": {
      "setNestedAttrs": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1SetNestedAttr:R1SetNestedAttr"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "setNestedAttrs": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/R1SetNestedAttr:R1SetNestedAttr"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1SetNestedAttr:R1SetNestedAttr": {
      "properties": {
        "x": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimMapNestedAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"map_nested_attr": schema.MapNestedAttribute{
					Optional: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"x": schema.StringAttribute{Optional: true},
						},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "map_nested_attr": {
        "element": {
          "schema": {
            "element": {
              "resource": {
                "x": {
                  "optional": true,
                  "type": 4
                }
              }
            },
            "type": 6
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "mapNestedAttr": {
        "type": "object",
        "additionalProperties": {
          "$ref": "#/types/testprov:index/R1MapNestedAttr:R1MapNestedAttr"
        }
      }
    },
    "inputProperties": {
      "mapNestedAttr": {
        "type": "object",
        "additionalProperties": {
          "$ref": "#/types/testprov:index/R1MapNestedAttr:R1MapNestedAttr"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "mapNestedAttr": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/testprov:index/R1MapNestedAttr:R1MapNestedAttr"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1MapNestedAttr:R1MapNestedAttr": {
      "properties": {
        "x": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimSingleNestedAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"single_nested_attr": schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						"x": schema.StringAttribute{Optional: true},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "single_nested_attr": {
        "element": {
          "resource": {
            "x": {
              "optional": true,
              "type": 4
            }
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "singleNestedAttr": {
        "$ref": "#/types/testprov:index/R1SingleNestedAttr:R1SingleNestedAttr"
      }
    },
    "inputProperties": {
      "singleNestedAttr": {
        "$ref": "#/types/testprov:index/R1SingleNestedAttr:R1SingleNestedAttr"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "singleNestedAttr": {
          "$ref": "#/types/testprov:index/R1SingleNestedAttr:R1SingleNestedAttr"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1SingleNestedAttr:R1SingleNestedAttr": {
      "properties": {
        "x": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimObjectAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"obj_attr": schema.ObjectAttribute{
					Optional: true,
					AttributeTypes: map[string]attr.Type{
						"x": types.StringType,
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "obj_attr": {
        "element": {
          "resource": {
            "x": {
              "type": 4
            }
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "objAttr": {
        "$ref": "#/types/testprov:index/R1ObjAttr:R1ObjAttr"
      }
    },
    "inputProperties": {
      "objAttr": {
        "$ref": "#/types/testprov:index/R1ObjAttr:R1ObjAttr"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "objAttr": {
          "$ref": "#/types/testprov:index/R1ObjAttr:R1ObjAttr"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1ObjAttr:R1ObjAttr": {
      "properties": {
        "x": {
          "type": "string"
        }
      },
      "type": "object",
      "required": [
        "x"
      ]
    }
  }
}`),
	})
}

func TestShimDynamicAttr(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Attributes: map[string]schema.Attribute{
				"obj_attr": schema.DynamicAttribute{
					Optional: true,
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "obj_attr": {
        "optional": true,
        "type": 8
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "objAttr": {
        "$ref": "pulumi.json#/Any"
      }
    },
    "inputProperties": {
      "objAttr": {
        "$ref": "pulumi.json#/Any"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "objAttr": {
          "$ref": "pulumi.json#/Any"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
	})
}

func TestShimSingleNestedBlock(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Blocks: map[string]schema.Block{
				"blk": schema.SingleNestedBlock{
					Attributes: map[string]schema.Attribute{
						"a1": schema.Float64Attribute{Optional: true},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "blk": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
            }
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "blk": {
        "$ref": "#/types/testprov:index/R1Blk:R1Blk"
      }
    },
    "inputProperties": {
      "blk": {
        "$ref": "#/types/testprov:index/R1Blk:R1Blk"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "blk": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1Blk:R1Blk": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimListNestedBlock(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Blocks: map[string]schema.Block{
				"blk": schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"a1": schema.Float64Attribute{Optional: true},
						},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "blk": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
            }
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      }
    },
    "inputProperties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "blks": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/R1Blk:R1Blk"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1Blk:R1Blk": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

// The bridge attempts some heuristics to infer listvalidator.SizeAtMost(1) and apply flattening. It is unclear how
// often it is used but there are non-0 actual examples, such as data_storage on elasticache serverless_cache in AWS.
func TestShimListNestedFlattenedBlock(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Blocks: map[string]schema.Block{
				"blk": schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"a1": schema.Float64Attribute{Optional: true},
						},
					},
					Validators: []validator.List{
						listvalidator.SizeAtMost(1),
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "blk": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
            }
          }
        },
        "maxItems": 1,
        "optional": true,
        "type": 5
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "blk": {
        "$ref": "#/types/testprov:index/R1Blk:R1Blk"
      }
    },
    "inputProperties": {
      "blk": {
        "$ref": "#/types/testprov:index/R1Blk:R1Blk"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "blk": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1Blk:R1Blk": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

func TestShimSetNestedBlock(t *testing.T) {
	t.Parallel()
	checkShim(t, shimTestCase{
		stdProvider(schema.Schema{
			Blocks: map[string]schema.Block{
				"blk": schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"a1": schema.Float64Attribute{Optional: true},
						},
					},
				},
			},
		}),
		autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "blk": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
            }
          }
        },
        "optional": true,
        "type": 7
      }
    }
  }
}`),
		autogold.Expect(`{
  "resource": {
    "properties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      }
    },
    "inputProperties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1Blk:R1Blk"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "blks": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/R1Blk:R1Blk"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1Blk:R1Blk": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
	})
}

type shimTestCase struct {
	provider     provider.Provider
	expect       autogold.Value // expected prettified shim.Schema representation
	expectSchema autogold.Value // expected corresponding Pulumi Package Schema extract
}

func checkShim(t *testing.T, tc shimTestCase) {
	shimmedProvider := schemashim.ShimSchemaOnlyProvider(context.Background(), tc.provider)

	m := tfbridge.MarshalProvider(shimmedProvider)
	bytes, err := json.Marshal(m)
	require.NoError(t, err)

	var pretty map[string]any
	err = json.Unmarshal(bytes, &pretty)
	require.NoError(t, err)

	prettyBytes, err := json.MarshalIndent(pretty, "", "  ")
	require.NoError(t, err)

	tc.expect.Equal(t, string(prettyBytes))

	rtok := "testprov:index:R1"

	info := info.Provider{
		Name: "testprov",
		P:    shimmedProvider,
		Resources: map[string]*info.Resource{
			"testprov_r1": {
				Tok: tokens.Type(rtok),
			},
		},
	}

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	pSpec, err := tfgen.GenerateSchema(info, nilSink)
	require.NoError(t, err)

	type miniSpec struct {
		Resource any `json:"resource"`
		Types    any `json:"types"`
	}

	ms := miniSpec{
		Resource: pSpec.Resources[rtok],
		Types:    pSpec.Types,
	}

	prettySpec, err := json.MarshalIndent(ms, "", "  ")
	require.NoError(t, err)

	tc.expectSchema.Equal(t, string(prettySpec))
}

func stdProvider(resourceSchema schema.Schema) *pb.Provider {
	return &pb.Provider{
		TypeName: "testprov",
		AllResources: []pb.Resource{{
			Name:           "r1",
			ResourceSchema: resourceSchema,
		}},
	}
}
