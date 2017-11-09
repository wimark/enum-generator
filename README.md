# README #

This enum-generator is intended to generate golang source code for string-based enum types and enum types with associated data from description in TOML.

## How to use ##

This command:

    enum-generator -enable-json -enable-bson -package=mypackage < enums.toml > enums.go

will make source file enums.go for "mypackage" Go package with enum types defined in enums.toml, with JSON and BSON (un)marshalling functions included.

Flags:

 - -package=packagename - sets name "packagename" for a package ("main" if not set)
 - -enable-json - JSON (un)marshalling functions for generated types are included to source code
 - -enable-bson - same for BSON

TOML structure sample:

    [Figure]
    default = "Dot"
    [Figure.variants]
    Rectangle = "rect"
    Circle    = "circle"
    Dot       = "dot"

    [FigureInfo]
    constraint = "Figure"
    [FigureInfo.Variants]
    Rectangle = "RectangleData"
    Circle    = "int"
    "Dot"     = "null"`
