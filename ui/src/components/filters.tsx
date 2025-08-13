'use client'

import { useState } from 'react'
import { Filter, X, ChevronDown } from 'lucide-react'

export interface FilterField {
  key: string
  label: string
  type: 'text' | 'select' | 'date' | 'daterange'
  options?: { value: string; label: string }[]
  placeholder?: string
}

export interface FilterValue {
  [key: string]: string | undefined
}

export interface FilterMeta {
  applied: { [key: string]: any }
  count: number
}

export interface FiltersProps {
  fields: FilterField[]
  values: FilterValue
  onChange: (values: FilterValue) => void
  onClear: () => void
  filterMeta?: FilterMeta
  className?: string
}

export function Filters({ fields, values, onChange, onClear, filterMeta, className }: FiltersProps) {
  const [isOpen, setIsOpen] = useState(false)

  const handleFieldChange = (key: string, value: string) => {
    onChange({
      ...values,
      [key]: value || undefined,
    })
  }

  const handleClearField = (key: string) => {
    const newValues = { ...values }
    delete newValues[key]
    onChange(newValues)
  }

  const activeFilters = Object.entries(values).filter(([_, value]) => value !== undefined && value !== '')
  const hasActiveFilters = activeFilters.length > 0

  const renderField = (field: FilterField) => {
    const value = values[field.key] || ''

    switch (field.type) {
      case 'select':
        return (
          <select
            value={value}
            onChange={(e) => handleFieldChange(field.key, e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white"
          >
            <option value="">{field.placeholder || `Select ${field.label}`}</option>
            {field.options?.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        )

      case 'date':
        return (
          <input
            type="date"
            value={value}
            onChange={(e) => handleFieldChange(field.key, e.target.value)}
            placeholder={field.placeholder}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        )

      case 'daterange':
        return (
          <div className="flex space-x-2">
            <input
              type="date"
              value={value.split(',')[0] || ''}
              onChange={(e) => {
                const endDate = value.split(',')[1] || ''
                handleFieldChange(field.key, `${e.target.value}${endDate ? ',' + endDate : ''}`)
              }}
              placeholder="Start date"
              className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
            <input
              type="date"
              value={value.split(',')[1] || ''}
              onChange={(e) => {
                const startDate = value.split(',')[0] || ''
                handleFieldChange(field.key, `${startDate}${startDate ? ',' : ''}${e.target.value}`)
              }}
              placeholder="End date"
              className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>
        )

      case 'text':
      default:
        return (
          <input
            type="text"
            value={value}
            onChange={(e) => handleFieldChange(field.key, e.target.value)}
            placeholder={field.placeholder || `Filter by ${field.label.toLowerCase()}`}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        )
    }
  }

  return (
    <div className={className}>
      <div className="bg-white border border-gray-200 rounded-lg shadow-sm">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setIsOpen(!isOpen)}
              className="flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              <Filter className="h-4 w-4 mr-2" />
              Filters
              <ChevronDown className={`h-4 w-4 ml-2 transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`} />
            </button>

            {filterMeta && (
              <div className="text-sm text-gray-600">
                {filterMeta.count} results {hasActiveFilters && '(filtered)'}
              </div>
            )}
          </div>

          {hasActiveFilters && (
            <button
              onClick={onClear}
              className="px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              Clear all
            </button>
          )}
        </div>

        {/* Active filters badges */}
        {hasActiveFilters && (
          <div className="flex flex-wrap gap-2 p-4 border-b border-gray-200 bg-gray-50">
            {activeFilters.map(([key, value]) => {
              const field = fields.find(f => f.key === key)
              if (!field || !value) return null

              let displayValue = value
              if (field.type === 'select') {
                const option = field.options?.find(opt => opt.value === value)
                displayValue = option?.label || value
              }

              return (
                <div key={key} className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md">
                  <span className="font-semibold">{field.label}:</span>
                  <span>{displayValue}</span>
                  <button
                    onClick={() => handleClearField(key)}
                    className="ml-1 text-gray-400 hover:text-gray-600 focus:outline-none"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </div>
              )
            })}
          </div>
        )}

        {/* Filter form */}
        {isOpen && (
          <div className="p-4">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {fields.map((field) => (
                <div key={field.key} className="space-y-2">
                  <label htmlFor={field.key} className="block text-sm font-medium text-gray-700">
                    {field.label}
                  </label>
                  {renderField(field)}
                </div>
              ))}
            </div>
            <div className="flex justify-end space-x-2 mt-6 pt-4 border-t border-gray-200">
              <button
                onClick={onClear}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              >
                Clear All
              </button>
              <button
                onClick={() => setIsOpen(false)}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                Apply Filters
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
