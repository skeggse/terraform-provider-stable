package provider

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var sharedT *testing.T

func TestGenerateNewResult(t *testing.T) {
	assert := assert.New(t)

	emptySet := &schema.Set{F: schema.HashString}
	aSet := schema.NewSet(schema.HashString, []interface{}{"a"})
	bSet := schema.NewSet(schema.HashString, []interface{}{"b"})
	abSet := schema.NewSet(schema.HashString, []interface{}{"a", "b"})
	acSet := schema.NewSet(schema.HashString, []interface{}{"a", "c"})

	if v, err := generateNewResult(0, emptySet, emptySet); assert.NoError(err) {
		assert.Nil(v)
	}

	_, err := generateNewResult(1, emptySet, emptySet)
	assert.Error(err)

	_, err = generateNewResult(2, aSet, abSet)
	assert.Error(err)

	if v, err := generateNewResult(1, aSet, emptySet); assert.NoError(err) {
		assert.True(aSet.Equal(v))
	}

	if v, err := generateNewResult(1, aSet, bSet); assert.NoError(err) {
		assert.True(aSet.Equal(v))
	}

	if v, err := generateNewResult(1, aSet, abSet); assert.NoError(err) {
		assert.True(aSet.Equal(v))
	}

	if v, err := generateNewResult(1, acSet, abSet); assert.NoError(err) {
		assert.True(aSet.Equal(v))
	}

	if v, err := generateNewResult(2, acSet, abSet); assert.NoError(err) {
		assert.True(acSet.Equal(v))
	}
}

func TestAccResourceSubset(t *testing.T) {
	t.Log("\n\nstart\n\n")
	sharedT = t
	resource.UnitTest(t, resource.TestCase{
		PreCheck:  func() {},
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "stable_subset" "bad" {
					input       = ["a", "b"]
					subset_size = 3
				}
				`,
				ExpectError: regexp.MustCompile("need as many input items as subset_count"),
			},
			{
				Config: `
				resource "stable_unknown" "i" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h"]
				}
				resource "stable_subset" "o" {
					input       = stable_unknown.i.result
					subset_size = 3
				}
				resource "stable_subset" "res" {
					input       = ["a", "b", "c", "d", "e"]
					subset_size = 3
				}
				resource "stable_subset" "res2" {
					input       = stable_subset.res.result
					subset_size = 2
				}
				`,
				Check: func(s *terraform.State) error {
					t.Logf("checking long, %v", t)
					if _, err := testPulledFromGiven("stable_subset.o", []interface{}{"a", "b", "c", "d", "e", "f", "g", "h"}, 3)(s); err != nil {
						return err
					}

					extracted, err := testPulledFromGiven("stable_subset.res", []interface{}{"a", "b", "c", "d"}, 3)(s)
					if err != nil {
						return err
					}
					if _, err := testPulledFromGiven("stable_subset.res2", extracted, 2)(s); err != nil {
						return err
					}
					return nil
				},
			},
		},
	})
}

func testPulledFromGiven(id string, wants_from []interface{}, count int) func(s *terraform.State) ([]interface{}, error) {
	return func(s *terraform.State) ([]interface{}, error) {
		rs, ok := s.RootModule().Resources[id]
		if !ok {
			return nil, fmt.Errorf("Not found: %s", id)
		}
		if rs.Primary.ID == "" {
			return nil, fmt.Errorf("No ID is set")
		}

		attrs := rs.Primary.Attributes

		consumeLen := strconv.Itoa(count)
		if attrs["subset_size"] != consumeLen {
			return nil, fmt.Errorf("expected subset_size of %s, got %s", consumeLen, attrs["subset_size"])
		}

		gotLen := attrs["result.#"]
		if gotLen != consumeLen {
			return nil, fmt.Errorf("got %s result items; want %s", gotLen, consumeLen)
		}

		wants := make(map[interface{}]int)
		for _, want := range wants_from {
			num, ok := wants[want]
			if !ok {
				num = 0
			}
			wants[want] = num + 1
		}

		var got []interface{}
		for i := 0; i < count; i++ {
			value := attrs[fmt.Sprintf("result.%d", i)]
			num, ok := wants[value]
			if !ok || num <= 0 {
				return nil, fmt.Errorf("got unexpected item %s", value)
			}
			got = append(got, value)
			wants[value] -= 1
		}

		remaining := 0
		for _, num := range wants {
			remaining += num
		}
		if remaining+count != len(wants_from) {
			return nil, fmt.Errorf("unexpected remainder %d", remaining)
		}

		return got, nil
	}
}
