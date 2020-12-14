HTMPL 5
=======

NAME
----

htmpl - templating with normal HTML

DESCRIPTION
-----------

HTMPL (contraction of "HTML template") is a templating language built on top of the HTML syntax; most HTML parsers can parse HTMPL without any modification.

VALUES
------

The exact set of value types is dependent on the implementation, but must include at least:

- bool - `true` or `false`
- empty - no value; called `null` or `nil` in many languages
- number - a number with some amount of decimal precision
- string - a sequence of Unicode code points
- array - an ordered sequence of values
- map - an unordered set of values associated with string keys

### Truthy and falsey values

For conditional expressions, values must be categorized into one of two types: truthy and falsey.

- A bool is truthy if it is `true` and falsey if it is `false
- An empty is always falsey
- A number is falsey if it is exactly equal to 0, otherwise it is truthy
- A string is falsey if it is of length 0, otherwise it is truthy
- An array is falsey if it is of length 0, otherwise it is truthy
- A map is always truthy

VARIABLES
---------

Variables are values with names.
Valid variable names include `.`, `$` and any sequence of alphanumeric or `_` characters.
When a template is evaluated, the variables `.` and `$` are set to the provided value.

### Variable paths

A variable path is a sequence of variable names separated by the `.` character.
If a variable path begins with a `.` character, it begins at the variable named `.`
The value of a variable path is retrieved by indexing the value at each step along the path.
Rules for indexing different types are as follows:

- Indexing a bool, empty, number or string always results in an empty
- Indexing an array results in the corresponding 0-indexed value if the key is a positive integer less than the array length, and an empty otherwise
- Indexing a map results in the corresponding value if the key is in the map, and an empty otherwise
- Indexing any type not defined by this document results in an implementation-defined value

If a variable path contains a section surrounded by square brackets (`[` and `]`), that section will be looked up independently and used as a key.

ELEMENT TYPES
-------------

HTMPL templates are constructed using HTML-style elements, starting with an opening tag and ending with a closing tag.
To ensure parsing is consistent between different HTML parsers, no element use the self-closing tag syntax.

### Variables

The simplest element is the variable substitution element, `<v>`.
The body of this element should be a variable path, the value of which will replace the `<v>` element when the template is evaluated.

If a `<v>` element has the optional `noescape` attribute, the expression's value will be interpreted as HTML.
Otherwise, it will be escaped so it displays as literal text in the final rendered page.

### Conditionals

Conditional branches can be performed using the `<if>` and `<nif>` elements.
These elements are required to have a single `v` attribute, containing a variable path used as the condition.
An `<if>` element will be replaced with its contents if the condition value is truthy, and nothing otherwise.
A `<nif>` element is identical to an `<if>` element except that the condition value is inverted.

### Loops

To loop over a collection of items, use the `<for>` element.
This element is required to have a single `v` attribute, containing an expression that results in the collection to be iterated over.
The `<for>` element will be replaced with its body once for each item in the collection.
The `.` variable will be set to the item on each iteration.

Iteration for types specified in this document is defined as follows:

- An empty contains nothing
- A bool, number or string is considered to "contain" itself, once
- An array contains the values in the array, in order
- A map contains the keys of the map, in any order (including randomized for each iteration)

### Let

Assignment of variables can be done using the `<let>` element.
This element is required to have a `var` attribute and a `val` attribute, containing the variable name to assign to and variable path to assign from, respectively.
After the element is closed, the variable binding reverts to its previous value.
