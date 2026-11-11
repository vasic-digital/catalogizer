import {
  COLLECTION_FIELD_OPTIONS,
  COLLECTION_OPERATORS,
  COLLECTION_TEMPLATES,
  MEDIA_TYPE_OPTIONS,
  QUALITY_OPTIONS,
  GENRE_OPTIONS,
  DECADE_OPTIONS,
  RESOLUTION_OPTIONS,
  LANGUAGE_OPTIONS,
  getFieldOptions,
  getFieldLabel,
  getFieldType,
  validateRule,
  validateRules,
} from '../collectionRules'

describe('collectionRules', () => {
  describe('COLLECTION_FIELD_OPTIONS', () => {
    it('contains expected fields', () => {
      const fieldValues = COLLECTION_FIELD_OPTIONS.map(f => f.value)
      expect(fieldValues).toContain('title')
      expect(fieldValues).toContain('artist')
      expect(fieldValues).toContain('genre')
      expect(fieldValues).toContain('year')
      expect(fieldValues).toContain('media_type')
      expect(fieldValues).toContain('rating')
    })

    it('each field has value, label, and type', () => {
      COLLECTION_FIELD_OPTIONS.forEach(field => {
        expect(field).toHaveProperty('value')
        expect(field).toHaveProperty('label')
        expect(field).toHaveProperty('type')
        expect(typeof field.value).toBe('string')
        expect(typeof field.label).toBe('string')
      })
    })
  })

  describe('COLLECTION_OPERATORS', () => {
    it('has operators for text, number, date, select, multiselect, boolean types', () => {
      expect(COLLECTION_OPERATORS).toHaveProperty('text')
      expect(COLLECTION_OPERATORS).toHaveProperty('number')
      expect(COLLECTION_OPERATORS).toHaveProperty('date')
      expect(COLLECTION_OPERATORS).toHaveProperty('select')
      expect(COLLECTION_OPERATORS).toHaveProperty('multiselect')
      expect(COLLECTION_OPERATORS).toHaveProperty('boolean')
    })

    it('text operators include contains and equals', () => {
      const textValues = COLLECTION_OPERATORS.text.map(o => o.value)
      expect(textValues).toContain('contains')
      expect(textValues).toContain('equals')
      expect(textValues).toContain('starts_with')
    })

    it('number operators include comparison operators', () => {
      const numValues = COLLECTION_OPERATORS.number.map(o => o.value)
      expect(numValues).toContain('greater_than')
      expect(numValues).toContain('less_than')
      expect(numValues).toContain('between')
    })

    it('boolean operators include is_true and is_false', () => {
      const boolValues = COLLECTION_OPERATORS.boolean.map(o => o.value)
      expect(boolValues).toContain('is_true')
      expect(boolValues).toContain('is_false')
    })
  })

  describe('COLLECTION_TEMPLATES', () => {
    it('contains pre-defined templates', () => {
      expect(COLLECTION_TEMPLATES.length).toBeGreaterThan(0)
    })

    it('each template has required fields', () => {
      COLLECTION_TEMPLATES.forEach(template => {
        expect(template).toHaveProperty('id')
        expect(template).toHaveProperty('name')
        expect(template).toHaveProperty('description')
        expect(template).toHaveProperty('category')
        expect(template).toHaveProperty('rules')
        expect(template.rules.length).toBeGreaterThan(0)
      })
    })

    it('includes recently_added template', () => {
      const recentTemplate = COLLECTION_TEMPLATES.find(t => t.id === 'recently_added')
      expect(recentTemplate).toBeDefined()
      expect(recentTemplate!.name).toBe('Recently Added')
    })
  })

  describe('getFieldOptions', () => {
    it('returns media type options for media_type field', () => {
      const options = getFieldOptions('media_type')
      expect(options).toEqual(MEDIA_TYPE_OPTIONS)
    })

    it('returns quality options for quality field', () => {
      const options = getFieldOptions('quality')
      expect(options).toEqual(QUALITY_OPTIONS)
    })

    it('returns genre options for genre field', () => {
      const options = getFieldOptions('genre')
      expect(options).toEqual(GENRE_OPTIONS)
    })

    it('returns decade options for decade field', () => {
      const options = getFieldOptions('decade')
      expect(options).toEqual(DECADE_OPTIONS)
    })

    it('returns resolution options for resolution field', () => {
      const options = getFieldOptions('resolution')
      expect(options).toEqual(RESOLUTION_OPTIONS)
    })

    it('returns language options for language field', () => {
      const options = getFieldOptions('language')
      expect(options).toEqual(LANGUAGE_OPTIONS)
    })

    it('returns tags options for tags field', () => {
      const options = getFieldOptions('tags')
      expect(options.length).toBeGreaterThan(0)
      expect(options[0]).toHaveProperty('value')
      expect(options[0]).toHaveProperty('label')
    })

    it('returns empty array for unknown field', () => {
      const options = getFieldOptions('unknown_field')
      expect(options).toEqual([])
    })
  })

  describe('getFieldLabel', () => {
    it('returns label for known field', () => {
      expect(getFieldLabel('title')).toBe('Title')
      expect(getFieldLabel('artist')).toBe('Artist')
      expect(getFieldLabel('media_type')).toBe('Media Type')
    })

    it('returns the value itself for unknown field', () => {
      expect(getFieldLabel('unknown_field')).toBe('unknown_field')
    })
  })

  describe('getFieldType', () => {
    it('returns correct type for known fields', () => {
      expect(getFieldType('title')).toBe('text')
      expect(getFieldType('year')).toBe('number')
      expect(getFieldType('genre')).toBe('select')
      expect(getFieldType('date_added')).toBe('date')
      expect(getFieldType('is_favorite')).toBe('boolean')
      expect(getFieldType('tags')).toBe('multiselect')
    })

    it('returns text for unknown field', () => {
      expect(getFieldType('unknown_field')).toBe('text')
    })
  })

  describe('validateRule', () => {
    it('returns no errors for valid rule', () => {
      const rule = {
        id: '1',
        field: 'title',
        operator: 'contains',
        value: 'test',
        field_type: 'text' as const,
        label: 'Title',
      }
      const errors = validateRule(rule)
      expect(errors).toEqual([])
    })

    it('returns error when field is missing', () => {
      const rule = {
        id: '1',
        field: '',
        operator: 'contains',
        value: 'test',
        field_type: 'text' as const,
        label: '',
      }
      const errors = validateRule(rule)
      expect(errors).toContain('Field is required')
    })

    it('returns error when operator is missing', () => {
      const rule = {
        id: '1',
        field: 'title',
        operator: '',
        value: 'test',
        field_type: 'text' as const,
        label: 'Title',
      }
      const errors = validateRule(rule)
      expect(errors).toContain('Operator is required')
    })

    it('returns error when value is required but missing', () => {
      const rule = {
        id: '1',
        field: 'title',
        operator: 'contains',
        value: '',
        field_type: 'text' as const,
        label: 'Title',
      }
      const errors = validateRule(rule)
      expect(errors).toContain('Value is required for this operator')
    })

    it('does not require value for is_empty operator', () => {
      const rule = {
        id: '1',
        field: 'title',
        operator: 'is_empty',
        value: null,
        field_type: 'text' as const,
        label: 'Title',
      }
      const errors = validateRule(rule)
      expect(errors).not.toContain('Value is required for this operator')
    })

    it('does not require value for is_true operator', () => {
      const rule = {
        id: '1',
        field: 'is_favorite',
        operator: 'is_true',
        value: null,
        field_type: 'boolean' as const,
        label: 'Is Favorite',
      }
      const errors = validateRule(rule)
      expect(errors).not.toContain('Value is required for this operator')
    })

    it('does not require value for date period operators', () => {
      const rule = {
        id: '1',
        field: 'date_added',
        operator: 'last_30_days',
        value: null,
        field_type: 'date' as const,
        label: 'Date Added',
      }
      const errors = validateRule(rule)
      expect(errors).not.toContain('Value is required for this operator')
    })

    it('returns error for non-numeric value on number field', () => {
      const rule = {
        id: '1',
        field: 'year',
        operator: 'equals',
        value: 'not-a-number',
        field_type: 'number' as const,
        label: 'Year',
      }
      const errors = validateRule(rule)
      expect(errors).toContain('Value must be a number')
    })

    it('returns error for invalid date between range', () => {
      const rule = {
        id: '1',
        field: 'date_added',
        operator: 'between',
        value: ['invalid-date'],
        field_type: 'date' as const,
        label: 'Date Added',
      }
      const errors = validateRule(rule)
      expect(errors).toContain('Date range must have 2 values')
    })

    // Characterization (CAT-QA-SONAR-2 stage 2): date branches pinned before the complexity refactor.
    const dateRule = (operator: string, value: unknown) => ({
      id: '1', field: 'date_added', operator, value, field_type: 'date' as const, label: 'Date Added',
    })

    it('flags an unparsable date inside a valid-length between range', () => {
      expect(validateRule(dateRule('between', ['2024-01-01', 'nope']) as never)).toEqual(['Invalid date format in range'])
      expect(validateRule(dateRule('not_between', ['2024-01-01', '2024-02-01']) as never)).toEqual([])
    })

    it('flags a single unparsable date, accepts a valid one, ignores relative-date operators', () => {
      expect(validateRule(dateRule('equals', 'nope') as never)).toEqual(['Invalid date format'])
      expect(validateRule(dateRule('equals', '2024-01-01') as never)).toEqual([])
      expect(validateRule(dateRule('this_week', 'nope') as never)).toEqual([])
      expect(validateRule(dateRule('last_90_days', 'nope') as never)).toEqual([])
    })

    it('requires a value unless the operator is value-less', () => {
      expect(validateRule(dateRule('equals', '') as never)).toEqual(['Value is required for this operator', 'Invalid date format'])
      expect(validateRule(dateRule('is_empty', '') as never)).toEqual(['Invalid date format'])
      expect(validateRule(dateRule('last_30_days', undefined) as never)).toEqual([])
    })
  })

  describe('validateRules', () => {
    it('returns error when rules array is empty', () => {
      const errors = validateRules([])
      expect(errors).toContain('At least one rule is required')
    })

    it('returns no errors for valid rules array', () => {
      const rules = [
        {
          id: '1',
          field: 'title',
          operator: 'contains',
          value: 'test',
          field_type: 'text' as const,
          label: 'Title',
        },
      ]
      const errors = validateRules(rules)
      expect(errors).toEqual([])
    })

    it('returns indexed error messages for invalid rules', () => {
      const rules = [
        {
          id: '1',
          field: '',
          operator: 'contains',
          value: 'test',
          field_type: 'text' as const,
          label: '',
        },
      ]
      const errors = validateRules(rules)
      expect(errors.length).toBeGreaterThan(0)
      expect(errors[0]).toContain('Rule 1:')
    })
  })
})
