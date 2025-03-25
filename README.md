BCL - Block Config Language
===========================

BCL is - another - human centric general configurateion language.

It is designed around the specific case of J5 Schemas, both to define [J5
Schemas](https://github.com/pentops/j5), and to be a j5-schema driven
configuration language for other purposes.

Goals:
 - All about the humans reading and writing.
 - Ability to extend the language over time.
 - Support 'Doc' blocks, large (markdown) text bodies attached to elements
 - Familiar for `{}` style programmers. 
 - Small, easy to learn

### Why this and not X?

"Because I wanted to see if I could" is probably the most honest answer, but
there were a few things missing in the available languages:

- [UCL](https://github.com/vstakhov/libucl) is very close to what I want, and
  with macros, BCL could probably even be implemented in UCL. ... I found UCL
  while writing these docs and linking to HCL, no regrets, it doesn't have a
  doc block.

- [HCL](https://github.com/hashicorp/hcl) would suit well, and has really nice
  elements, but I don't want to get involved in whatever copyleft and right
  problem is happening there.

- JSON Schema is good, and close-ish to what we want for the schema definitions.
  Swagger/OAS, however, is too based in REST and ends up being an exercise in
  duck punching.

- JSON - no comments, no set structure

- JSON + Schema + extensions for comments: we are still inventing a 'language',
  but without control over the syntax. This applies for YAML, XML as well

- YAML - I love yaml, but my eyes are getting dim, I can't figure out how nested
  I am.

- TOML - Great for configuration, but it is really only designed for a few
  levels of nesting at best.

- PKL - looks interesting, but is typed in the config file itself rather than
  filling in a pre-defined schema.

There are single purpose languages which also compare pretty well:

- PROTO is cool, single-purpose - that purpose being API definitions, and the
  basis for J5, but the extensions are getting out of hand. The 'defaults' we
  want to define on all fields are all extension options, so we would end up
  writing a validator anyway.

- Nginx Config format is awesome. Much like HCL, this will look familiar to
  anyone who has worked with Nginx.

## Status

Unstable and immature. (but enough about me...)

# Structure

BCL works in layers. I mean, all languages do but the API for this is available at each layer.

## Layer 1: Base Syntax

Defines the Assign and Block structures from a string. Has no specific
structure or file format. The 'lexer' and 'parser' parts of a language.

The API exposes tools to BYO schema and parse it directly, defining handlers
for Assignments, Blocks and Docs.

BYO Schema, the API allows you to build tools to walk and valudate schemas,
including line errors for both syntax and schema issues.

## Layer 2: Schema

Defining the types of blocks as J5 Schemas.

Deifned in `.proto` for now becuase this doesn't work yey, but passed in to the
library as compiled so when it does work they will also be defined in BCL

## Layer 3: Modules

Similar to Go and Buf-Proto, the directory of a file specifies a 'package'.

Like Go, but not proto, the file name does not matter, you can freely move
content between files without changing the result. Imports import the package,
not the file, and there is no 'index' file like js modules.

The build in directives `export`, `import`, `partial` and `include` allow
merging and reference between elements as a nested structure.

# Layer 1, Syntax

```bcl
// Assignment
foo = "bar"

// Directive
foo bar "baz"

// Doc
| documentation
| multiline
| string

/*
  multi line comment
*/


// Block
foo bar baz {
  | documentation

  key = value

  // Nested block
  qux {
    key = value
  }
}

```

### Base elements

An `ident` starts with a unicode letter character, upper and lower, followed
by letters or numbers. `[a-zA-Z][a-zA-Z0-9]*`

A `reference` is a series of `ident` separated by periods. `ident(.ident)*`

A `literal` is a string, number, or boolean.
 - Strings, quoted with ""
 - Numbers, integers or floats specified 1.1 or 1
 - Booleans, true or false with no quotes
 - null, for un-setting things like partial overrides

Context defines the type of the literal, so 1 and 1.1 are both valid for floats
(i.e. you don't have to write 1.0)

Strings may span multiple lines, by escaping the end of line, and may also
escape quotes.

```j5
key = "This is a string"
key = "This \
is a string"
key = "This is a "string""
```
### Comment

Comments are C-style, `//` for single line, `/* */` for multi-line.

### Assignment

```j5
key = value
```

Keys are 'reference' type.
Values are 'literal' type.

### Directive

```j5
keyword value
package foo.bar
field foo string
```

The available keywords for directives are context dependent, and built in.
The context and keyword defines the number and type of the arguments.

Keys are 'reference' type.
Values are 'literal' type.

### Block

The root of the document is a body, which is a series of Assignments,
Directives, comments and Definitions.

Elements use curly braces `{}` to define the body.

```j5
field foo string{
  // ... body elements
}
```

Skipping the body is equivalent to an empty body, if there is nothing to define
the body you can just skip it.

```j5
field foo string
field foo string {
}
```

### Doc

Docs are like multi-line comments, but specifically used to describe
elements for documentation in generated code and schemas, rather than comment
about the code.

Docs are valid in all Block Bodies (including the root), but specific schemas may restrict.

Docs can be specified in two ways:

```j5
field foo string | Inline Doc on a single line
```

```j5
field foo string {
  | Doc in the body, which can span multiple lines
  | but must be specified at the start of the block.
}
```

Inline descriptions don't work with body blocks,
i.e. the following is **not valid**

```j5
field foo string { | Inline
    // ... body elements
}
```

In Module mode, Docs are not valid at the file level, we don't want to
end up with the `doc.go` thing, that's what README.md is for.

The intention (and syntax hilighting) is that descriptions are markdown
documents, however in Layer 1 these are just big strings.

In module mode, start carefully until we can build some validation rules:

- Using **bold**, *italic* and `code`: Good to go.
- Headings: Avoid until we figure out what the nesting would be. It certainly
  wouldn't make sense to have a full document with headings in the description
  of an Enum Option, for example.
- Paragraphs: In the right context, paragraphs are fine, using a blank like.
- Block-Quotes, Block Code: Not yet
- Lists: Use sparingly in the right context, nest freely
- Tables: Not yet, except for the special emum case.

Links... Are going to be a whole thing. Linking to wikipedia is fine, but the
path structure relative to the docs is not yet defined.

A special syntax similar to autolink for element links is pending, so when we
refer to an [Foo] in a [Bar] we can link to the definition of Foo
automatically.

Sequence and State Diagrams in mermaid are coming, probably more directly than
the documentation comments.

## Layer 2: Schema

> Status: Work in progress.

## Layer 3: Modules

> Status: Future.

The language extensions use `.`, so are compatible with macros in UCL syntax helpers.

### `.partial` and `.include`

Defines a partial entity, which can be merged into another entity of the same
type.

```j5
.partial field cusip {
    required
    validate.regex = "^[A-Z0-9]{9}$"
}

field foo string {
    include cusip
}

field bar string {
    .include cusip
    validate.regex = null
}
```

The resulting Foo is required with the regex.

Bar will still be required (as useless as that is) but will not have the regex.

There is no syntax to nullify a directive. // TODO: 'unset' directive?


### `.import` and `.export`

Imports all exported elements from another file under the given namespace.

```j5
.import foo.bar
.import foo.bar as baz
```

The exported elements are available as `bar.element` or `baz.element` respectively, rather than requiring the full namespace to be repeated.

Exports all elements for use by other namespaces.
By default, elements defined within a namespace are available to that namespace
and its children, but not to other namespaces.

```bcl

// namespace/foo.j5
object foo {
  .export
  field bar string
}

partial object baseline {
  field createdAt timestamp
}

// other/bar.js
.import namespace.foo as baz

object qux {
  .include baz.baseline

  field bar_ref {
    ref baz.foo
  }
}
```

