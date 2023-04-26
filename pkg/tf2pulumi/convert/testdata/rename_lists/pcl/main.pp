// Check that lists as attributes are handled correctly
resource listAttrResource "renames:index/index:resource" {
    theList = [
        {
            number = 1
        },
        {
            number = 2
        }    ]
}
// Check that lists as blocks are handled correctly
resource listBlockResource "renames:index/index:resource" {
    theList = [
        {
            number = 1
        },
        {
            number = 2
        }    ]
}
