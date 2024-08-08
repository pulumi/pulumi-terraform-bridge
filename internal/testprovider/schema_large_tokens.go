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

package testprovider

import (
	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go/service/quicksight"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func ProviderLargeTokens() *schema.Provider {
	resourceTemplate := func() *schema.Resource {
		return &schema.Resource{
			Schema:      resourceQuicksightTemplate(),
			Description: "Quicksight template",
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_quicksight_template": resourceTemplate(),
		},
	}
}

func resourceQuicksightTemplate() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"definition": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Optional: true,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"sheets": {
						Type:     schema.TypeList,
						MinItems: 1,
						MaxItems: 20,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"sheet_id": idSchema(),
								"content_type": {
									Type:         schema.TypeString,
									Optional:     true,
									Computed:     true,
									ValidateFunc: validation.StringInSlice([]string{"abc"}, false),
								},
								"description": stringSchema(false, validation.StringLenBetween(1, 1024)),
								// "filter_controls":       filterControlsSchema(),
								// "layouts":               layoutSchema(),
								"name": stringSchema(false, validation.StringLenBetween(1, 2048)),
								// "parameter_controls":    parameterControlsSchema(),
								// "sheet_control_layouts": sheetControlLayoutsSchema(),
								"text_boxes": {
									Type:     schema.TypeList,
									MinItems: 1,
									MaxItems: 100,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"sheet_text_box_id": idSchema(),
											"content":           stringSchema(false, validation.StringLenBetween(1, 150000)),
										},
									},
								},
								"title": stringSchema(false, validation.StringLenBetween(1, 1024)),
								"visuals": {
									Type:     schema.TypeList,
									MinItems: 1,
									MaxItems: 50,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											// "bar_chart_visual":      barCharVisualSchema(),
											// "box_plot_visual":       boxPlotVisualSchema(),
											// "combo_chart_visual":    comboChartVisualSchema(),
											// "custom_content_visual": customContentVisualSchema(),
											// "empty_visual":          emptyVisualSchema(),
											// "filled_map_visual":     filledMapVisualSchema(),
											// "funnel_chart_visual":   funnelChartVisualSchema(),
											// "gauge_chart_visual":    gaugeChartVisualSchema(),
											"geospatial_map_visual": {
												Type:     schema.TypeList,
												Optional: true,
												MinItems: 1,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"visual_id": idSchema(),
														"actions":   visualCustomActionsSchema(10),
														"chart_configuration": {
															Type:     schema.TypeList,
															Optional: true,
															MinItems: 1,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"field_wells": {
																		Type:     schema.TypeList,
																		Optional: true,
																		MinItems: 1,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"geospatial_map_aggregated_field_wells": {
																					Type:     schema.TypeList,
																					Optional: true,
																					MinItems: 1,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"colors":     dimensionFieldSchema(200),
																							"geospatial": dimensionFieldSchema(200),
																							// names.AttrValues: measureFieldSchema(measureFieldsMaxItems200),
																						},
																					},
																				},
																			},
																		},
																	},
																	"legend":            legendOptionsSchema(),
																	"map_style_options": geospatialMapStyleOptionsSchema(),
																	"point_style_options": {
																		Type:     schema.TypeList,
																		Optional: true,
																		MinItems: 1,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"cluster_marker_configuration": {
																					Type:     schema.TypeList,
																					Optional: true,
																					MinItems: 1,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"cluster_marker": {
																								Type:     schema.TypeList,
																								Optional: true,
																								MinItems: 1,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"simple_cluster_marker": {
																											Type:     schema.TypeList,
																											Optional: true,
																											MinItems: 1,
																											MaxItems: 1,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"color": stringSchema(
																														false,
																														validation.StringMatch(regexache.MustCompile(`^#[0-9A-F]{6}$`), ""),
																													),
																												},
																											},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																				"selected_point_style": stringSchema(
																					false,
																					validation.StringInSlice(quicksight.GeospatialSelectedPointStyle_Values(), false),
																				),
																			},
																		},
																	},
																	// "tooltip":        tooltipOptionsSchema(),
																	// "visual_palette": visualPaletteSchema(),
																	// "window_options": geospatialWindowOptionsSchema(),
																},
															},
														},
														// "column_hierarchies": columnHierarchiesSchema(),
														// "subtitle":           visualSubtitleLabelOptionsSchema(),
														// "title":              visualTitleLabelOptionsSchema(),
													},
												},
											},
											// "heat_map_visual":       heatMapVisualSchema(),
											// "histogram_visual":      histogramVisualSchema(),
											// "insight_visual":        insightVisualSchema(),
											// "kpi_visual":            kpiVisualSchema(),
											// "line_chart_visual":     lineChartVisualSchema(),
											// "pie_chart_visual":      pieChartVisualSchema(),
											// "pivot_table_visual":    pivotTableVisualSchema(),
											// "radar_chart_visual":    radarChartVisualSchema(),
											// "sankey_diagram_visual": sankeyDiagramVisualSchema(),
											// "scatter_plot_visual":   scatterPlotVisualSchema(),
											// "table_visual":          tableVisualSchema(),
											// "tree_map_visual":       treeMapVisualSchema(),
											// "waterfall_visual":      waterfallVisualSchema(),
											// "word_cloud_visual":     wordCloudVisualSchema(),
										},
									},
								},
							},
						},
					},
					"analysis_defaults": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"default_new_sheet_configuration": {
									Type:     schema.TypeList,
									Required: true,
									MinItems: 1,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"paginated_layout_configuration": {
												Type:     schema.TypeList,
												Optional: true,
												MinItems: 1,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"section_based": {
															Type:     schema.TypeList,
															Optional: true,
															MinItems: 1,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"canvas_size_options": {
																		Type:     schema.TypeList,
																		Required: true,
																		MinItems: 1,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"paper_canvas_size_options": {
																					Type:     schema.TypeList,
																					Optional: true,
																					MinItems: 1,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"paper_margin": {
																								Type:     schema.TypeList,
																								Optional: true,
																								MinItems: 1,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"bottom": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"left": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"right": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"top": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																									},
																								},
																							},
																							"paper_orientation": {
																								Type:     schema.TypeString,
																								Required: false,
																								Optional: true,
																								ValidateFunc: func(i interface{}, s string) ([]string, []error) {
																									return []string{}, []error{}
																								},
																							},
																							"paper_size": {
																								Type:     schema.TypeString,
																								Required: false,
																								Optional: true,
																								ValidateFunc: func(i interface{}, s string) ([]string, []error) {
																									return []string{}, []error{}
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
											"interactive_layout_configuration": {
												Type:     schema.TypeList,
												Optional: true,
												MinItems: 1,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"free_form": {
															Type:     schema.TypeList,
															Optional: true,
															MinItems: 1,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"canvas_size_options": {
																		Type:     schema.TypeList,
																		Required: true,
																		MinItems: 1,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"screen_canvas_size_options": {
																					Type:     schema.TypeList,
																					Optional: true,
																					MinItems: 1,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"optimized_view_port_width": {
																								Type:     schema.TypeString,
																								Required: true,
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					"data_set_configuration": {
						Type:     schema.TypeList,
						MaxItems: 30,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"column_group_schema_list": {
									Type:     schema.TypeList,
									MinItems: 1,
									MaxItems: 500,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"column_group_column_schema_list": {
												Type:     schema.TypeList,
												MinItems: 1,
												MaxItems: 500,
												Optional: true,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"name": {
															Type:     schema.TypeString,
															Optional: true,
														},
													},
												},
											},
											"name": {
												Type:     schema.TypeString,
												Optional: true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func stringSchema(
	required bool,
	//nolint:staticcheck
	validateFunc schema.SchemaValidateFunc,
) *schema.Schema {
	return &schema.Schema{
		Type:         schema.TypeString,
		Required:     required,
		Optional:     !required,
		ValidateFunc: validateFunc,
	}
}

func idSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
		ValidateFunc: validation.All(
			validation.StringLenBetween(1, 512),
			validation.StringMatch(
				regexache.MustCompile(`[\w\-]+`),
				"must contain only alphanumeric, hyphen, and underscore characters",
			),
		),
	}
}

func visualCustomActionsSchema(maxItems int) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: maxItems,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"action_operations": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 2,
					Required: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"filter_operation": {
								Type:     schema.TypeList,
								MinItems: 1,
								MaxItems: 1,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"selected_fields_configuration": {
											Type:     schema.TypeList,
											MinItems: 1,
											MaxItems: 1,
											Required: true,
											Elem: &schema.Resource{
												Schema: map[string]*schema.Schema{
													"selected_field_option": stringSchema(
														false,
														validation.StringInSlice(quicksight.SelectedFieldOptions_Values(), false),
													),
													"selected_fields": {
														Type:     schema.TypeList,
														Optional: true,
														MinItems: 1,
														MaxItems: 20,
														Elem: &schema.Schema{
															Type:         schema.TypeString,
															ValidateFunc: validation.StringLenBetween(1, 512),
														},
													},
												},
											},
										},
										"target_visuals_configuration": {
											Type:     schema.TypeList,
											MinItems: 1,
											MaxItems: 1,
											Required: true,
											Elem: &schema.Resource{
												Schema: map[string]*schema.Schema{
													"same_sheet_target_visual_configuration": {
														Type:     schema.TypeList,
														MinItems: 1,
														MaxItems: 1,
														Optional: true,
														Elem: &schema.Resource{
															Schema: map[string]*schema.Schema{
																"target_visual_option": stringSchema(
																	false,
																	validation.StringInSlice(quicksight.TargetVisualOptions_Values(), false),
																),
																"target_visuals": {
																	Type:     schema.TypeSet,
																	Optional: true,
																	MinItems: 1,
																	MaxItems: 30,
																	Elem:     &schema.Schema{Type: schema.TypeString},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							"navigation_operation": {
								Type:     schema.TypeList,
								MinItems: 1,
								MaxItems: 1,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"local_navigation_configuration": {
											Type:     schema.TypeList,
											MinItems: 1,
											MaxItems: 1,
											Optional: true,
											Elem: &schema.Resource{
												Schema: map[string]*schema.Schema{
													"target_sheet_id": idSchema(),
												},
											},
										},
									},
								},
							},
							"set_parameters_operation": {
								Type:     schema.TypeList,
								MinItems: 1,
								MaxItems: 1,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"parameter_value_configurations": {
											Type:     schema.TypeList,
											MinItems: 1,
											MaxItems: 200,
											Required: true,
											Elem: &schema.Resource{
												Schema: map[string]*schema.Schema{
													"destination_parameter_name": parameterNameSchema(true),
													"value": {
														Type:     schema.TypeList,
														MinItems: 1,
														MaxItems: 1,
														Required: true,
														Elem: &schema.Resource{
															Schema: map[string]*schema.Schema{
																"custom_values_configuration": {
																	Type:     schema.TypeList,
																	MinItems: 1,
																	MaxItems: 1,
																	Optional: true,
																	Elem: &schema.Resource{
																		Schema: map[string]*schema.Schema{
																			"custom_values": {
																				Type:     schema.TypeList,
																				MinItems: 1,
																				MaxItems: 1,
																				Required: true,
																				Elem: &schema.Resource{
																					Schema: map[string]*schema.Schema{
																						"date_time_values": {
																							Type:     schema.TypeList,
																							Optional: true,
																							MinItems: 1,
																							MaxItems: 50000,
																							Elem: &schema.Schema{
																								Type: schema.TypeString,
																								// ValidateFunc: verify.ValidUTCTimestamp,
																							},
																						},
																						"decimal_values": {
																							Type:     schema.TypeList,
																							Optional: true,
																							MinItems: 1,
																							MaxItems: 50000,
																							Elem: &schema.Schema{
																								Type: schema.TypeFloat,
																							},
																						},
																						"integer_values": {
																							Type:     schema.TypeList,
																							Optional: true,
																							MinItems: 1,
																							MaxItems: 50000,
																							Elem: &schema.Schema{
																								Type: schema.TypeInt,
																							},
																						},
																						"string_values": {
																							Type:     schema.TypeList,
																							Optional: true,
																							MinItems: 1,
																							MaxItems: 50000,
																							Elem: &schema.Schema{
																								Type: schema.TypeString,
																							},
																						},
																					},
																				},
																			},
																			"include_null_value": {
																				Type:     schema.TypeBool,
																				Optional: true,
																			},
																		},
																	},
																},
																"select_all_value_options": stringSchema(
																	false,
																	validation.StringInSlice(quicksight.SelectAllValueOptions_Values(), false),
																),
																"source_field": stringSchema(false, validation.StringLenBetween(1, 2048)),
																"source_parameter_name": {
																	Type:     schema.TypeString,
																	Optional: true,
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							"url_operation": {
								Type:     schema.TypeList,
								MinItems: 1,
								MaxItems: 1,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"url_target": stringSchema(
											true,
											validation.StringInSlice(quicksight.URLTargetConfiguration_Values(), false),
										),
										"url_template": stringSchema(true, validation.StringLenBetween(1, 2048)),
									},
								},
							},
						},
					},
				},
				"custom_action_id": idSchema(),
				"name":             stringSchema(true, validation.StringLenBetween(1, 256)),
				"trigger": stringSchema(
					true,
					validation.StringInSlice(quicksight.VisualCustomActionTrigger_Values(), false),
				),
				"status": stringSchema(true, validation.StringInSlice(quicksight.Status_Values(), false)),
			},
		},
	}
}

func parameterNameSchema(required bool) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeString,
		Required: required,
		Optional: !required,
		ValidateFunc: validation.All(
			validation.StringLenBetween(1, 2048),
			validation.StringMatch(regexache.MustCompile(`^[0-9A-Za-z]+$`), ""),
		),
	}
}

func dimensionFieldSchema(maxItems int) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: maxItems,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"categorical_dimension_field": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"column":               columnSchema(true),
							"field_id":             stringSchema(true, validation.StringLenBetween(1, 512)),
							"format_configuration": stringFormatConfigurationSchema(),
							"hierarchy_id":         stringSchema(false, validation.StringLenBetween(1, 512)),
						},
					},
				},
				"date_dimension_field": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"column":   columnSchema(true),
							"field_id": stringSchema(true, validation.StringLenBetween(1, 512)),
							"date_granularity": stringSchema(
								false,
								validation.StringInSlice(quicksight.TimeGranularity_Values(), false),
							),
							"format_configuration": dateTimeFormatConfigurationSchema(),
							"hierarchy_id":         stringSchema(false, validation.StringLenBetween(1, 512)),
						},
					},
				},
				"numerical_dimension_field": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"column":               columnSchema(true),
							"field_id":             stringSchema(true, validation.StringLenBetween(1, 512)),
							"format_configuration": numberFormatConfigurationSchema(),
							"hierarchy_id":         stringSchema(false, validation.StringLenBetween(1, 512)),
						},
					},
				},
			},
		},
	}
}

func columnSchema(required bool) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Required: required,
		Optional: !required,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"column_name":         stringSchema(true, validation.StringLenBetween(1, 128)),
				"data_set_identifier": stringSchema(true, validation.StringLenBetween(1, 2048)),
			},
		},
	}
}

func dateTimeFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"date_time_format":                stringSchema(false, validation.StringLenBetween(1, 128)),
				"null_value_format_configuration": nullValueConfigurationSchema(),
				"numeric_format_configuration":    numericFormatConfigurationSchema(),
			},
		},
	}
}
func nullValueConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"null_string": stringSchema(true, validation.StringLenBetween(1, 128)),
			},
		},
	}
}

func stringFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"null_value_format_configuration": nullValueConfigurationSchema(),
				"numeric_format_configuration":    numericFormatConfigurationSchema(),
			},
		},
	}
}

func numericFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"currency_display_format_configuration": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"decimal_places_configuration":    decimalPlacesConfigurationSchema(),
							"negative_value_configuration":    negativeValueConfigurationSchema(),
							"null_value_format_configuration": nullValueConfigurationSchema(),
							"number_scale": stringSchema(
								false,
								validation.StringInSlice(quicksight.NumberScale_Values(), false),
							),
							"prefix":                  stringSchema(false, validation.StringLenBetween(1, 128)),
							"separator_configuration": separatorConfigurationSchema(),
							"suffix":                  stringSchema(false, validation.StringLenBetween(1, 128)),
							"symbol": stringSchema(
								false,
								validation.StringMatch(regexache.MustCompile(`[A-Z]{3}`), "must be a 3 character currency symbol"),
							),
						},
					},
				},
				"number_display_format_configuration":     numberDisplayFormatConfigurationSchema(),
				"percentage_display_format_configuration": percentageDisplayFormatConfigurationSchema(),
			},
		},
	}
}
func decimalPlacesConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"decimal_places": {
					Type:         schema.TypeInt,
					Required:     true,
					ValidateFunc: validation.IntBetween(0, 20),
				},
			},
		},
	}
}

func negativeValueConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"display_mode": stringSchema(true, validation.StringInSlice(quicksight.NegativeValueDisplayMode_Values(), false)),
			},
		},
	}
}

func separatorConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"decimal_separator": stringSchema(
					false,
					validation.StringInSlice(quicksight.NumericSeparatorSymbol_Values(), false),
				),
				"thousands_separator": {
					Type:     schema.TypeList,
					MinItems: 1,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"symbol":     stringSchema(false, validation.StringInSlice(quicksight.NumericSeparatorSymbol_Values(), false)),
							"visibility": stringSchema(false, validation.StringInSlice(quicksight.Visibility_Values(), false)),
						},
					},
				},
			},
		},
	}
}

func numberDisplayFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"decimal_places_configuration":    decimalPlacesConfigurationSchema(),
				"negative_value_configuration":    negativeValueConfigurationSchema(),
				"null_value_format_configuration": nullValueConfigurationSchema(),
				"number_scale": stringSchema(
					false,
					validation.StringInSlice(quicksight.NumberScale_Values(), false),
				),
				"prefix":                  stringSchema(false, validation.StringLenBetween(1, 128)),
				"separator_configuration": separatorConfigurationSchema(),
				"suffix":                  stringSchema(false, validation.StringLenBetween(1, 128)),
			},
		},
	}
}
func percentageDisplayFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"decimal_places_configuration":    decimalPlacesConfigurationSchema(),
				"negative_value_configuration":    negativeValueConfigurationSchema(),
				"null_value_format_configuration": nullValueConfigurationSchema(),
				"prefix":                          stringSchema(false, validation.StringLenBetween(1, 128)),
				"separator_configuration":         separatorConfigurationSchema(),
				"suffix":                          stringSchema(false, validation.StringLenBetween(1, 128)),
			},
		},
	}
}

func numberFormatConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"numeric_format_configuration": numericFormatConfigurationSchema(),
			},
		},
	}
}

func legendOptionsSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MinItems: 1,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"height": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"position":   stringSchema(false, validation.StringInSlice(quicksight.LegendPosition_Values(), false)),
				"title":      labelOptionsSchema(),
				"visibility": stringSchema(false, validation.StringInSlice(quicksight.Visibility_Values(), false)),
				"width": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
}
func labelOptionsSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"custom_label": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"font_configuration": fontConfigurationSchema(),
				"visibility":         stringSchema(false, validation.StringInSlice(quicksight.Visibility_Values(), false)),
			},
		},
	}
}

func fontConfigurationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		MinItems: 1,
		MaxItems: 1,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"font_color":      stringSchema(false, validation.StringMatch(regexache.MustCompile(`^#[0-9A-F]{6}$`), "")),
				"font_decoration": stringSchema(false, validation.StringInSlice(quicksight.FontDecoration_Values(), false)),
				"font_size": {
					Type:     schema.TypeList,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"relative": stringSchema(false, validation.StringInSlice(quicksight.RelativeFontSize_Values(), false)),
						},
					},
				},
				"font_style": stringSchema(false, validation.StringInSlice(quicksight.FontStyle_Values(), false)),
				"font_weight": {
					Type:     schema.TypeList,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": stringSchema(false, validation.StringInSlice(quicksight.FontWeightName_Values(), false)),
						},
					},
				},
			},
		},
	}
}

func geospatialMapStyleOptionsSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MinItems: 1,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"base_map_style": stringSchema(false, validation.StringInSlice(quicksight.BaseMapStyleType_Values(), false)),
			},
		},
	}
}
