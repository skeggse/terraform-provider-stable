package provider

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceSubset() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "The list of strings from which to generate a stable subset. Only changes if the input ceases to contain one or more of the result values.",

		CreateContext: resourceSubsetCreate,
		Read:          schema.Noop,
		UpdateContext: resourceSubsetUpdate,
		Delete:        schema.RemoveFromState,

		Schema: map[string]*schema.Schema{
			"input": {
				Description: "The set of strings to shuffle.",
				Type:        schema.TypeSet,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"subset_size": {
				Description: "The size of the subset to generate",
				Type:        schema.TypeInt,
				Required:    true,
			},

			"result": {
				Description: "Random, stable subset of the set of strings given in `input`.",
				Type:        schema.TypeSet,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"id": {
				Description: "A static value used internally by Terraform, this should not be referenced in configurations.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
			if d.HasChange("result") {
				// Bail!
				return nil
			}
			subset_size, exists := d.GetOk("subset_size")
			if !exists {
				return nil
			}
			input, exists := d.GetOk("input")
			if !exists {
				// We can't estimate anything yet.
				return nil
			}
			new_result, err := generateNewResult(subset_size.(int), input.(*schema.Set), getPriorResult(d))
			if err != nil {
				return err
			}
			if new_result != nil {
				// TODO: avoid this List cast!
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/459
				d.SetNew("result", new_result.List())
			}
			return nil
		},
	}
}

func generateNewResult(subset_size int, input *schema.Set, prior_result *schema.Set) (*schema.Set, error) {
	if input_count := input.Len(); input_count < subset_size {
		return nil, fmt.Errorf("need as many input items as subset_count: %d < %d", input_count, subset_size)
	}
	// Remove items that aren't in `input` anymore.
	size_before_intersect := prior_result.Len()
	remaining_result := prior_result.Intersection(input)
	size_after_intersect := remaining_result.Len()
	num_add := subset_size - size_after_intersect
	if num_add == 0 {
		if size_before_intersect != subset_size {
			return remaining_result, nil
		}
		return nil, nil
	}

	if num_add < 0 {
		prior_items := remaining_result.List()
		keep_indexes := rand.Perm(len(prior_items))[:subset_size]
		new_result := &schema.Set{F: schema.HashString}
		for _, idx := range keep_indexes {
			new_result.Add(prior_items[idx])
		}
		return new_result, nil
	}

	sample_from := input.Difference(remaining_result).List()
	add_indexes := rand.Perm(len(sample_from))[:num_add]
	sharedT.Logf("indexes: %v", add_indexes)
	for _, idx := range add_indexes {
		remaining_result.Add(sample_from[idx])
	}
	return remaining_result, nil
}

type ResourceContainer interface {
	GetOk(key string) (interface{}, bool)
}

func getPriorResult(d ResourceContainer) *schema.Set {
	prior_result, exists := d.GetOk("result")
	if exists {
		return prior_result.(*schema.Set)
	}
	return &schema.Set{F: schema.HashString}
}

func resourceSubsetCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if !d.HasChange("result") {
		subset_size, exists := d.GetOk("subset_size")
		if !exists {
			return diag.Errorf("no subset_size provided")
		}
		input, exists := d.GetOk("input")
		if !exists {
			return diag.Errorf("no input provided")
		}
		new_result, err := generateNewResult(subset_size.(int), input.(*schema.Set), getPriorResult(d))
		if err != nil {
			return diag.FromErr(err)
		}
		if new_result != nil {
			d.Set("result", new_result)
		}
	}
	d.SetId("-")

	return nil
}

func resourceSubsetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return diag.Errorf("not implemented (update)")
}
