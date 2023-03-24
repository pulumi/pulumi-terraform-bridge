output nullOut {
    value = null
}
output numberOut {
    value = 0
}
output boolOut {
    value = true
}
output stringOut {
    value = "hello world"
}
output tupleOut {
    value = [1, 2, 3]
}
output numberOperatorsOut {
    value = -((1 + 2)) * 3 / 4 % 5
}
output boolOperatorsOut {
    value = !((true || false)) && true
}
output strObjectOut {
    value = {
        hello: "hallo"
        goodbye: "ha det"
    }
}
    aKey = "hello"
    aValue = -1
    aList = [1, 2, 3]
    aListOfMaps = [
        {
            x: [1, 2]
            y: [3, 4]
        },
        {
            x: [5, 6]
            y: [7, 8]
        }
    ]
output staticIndexOut {
    value = aList[1]
}
output dynamicIndexOut {
    value = aList[aValue]
}
output complexObjectOut {
    value = {
        a_tuple: ["a", "b", "c"]
        an_object: {
            literal_key: 1
            another_literal_key = 2
            "yet_another_literal_key": aValue
            # This doesn't translate correctly
            # (local.a_key) = 4
        }
        ambiguous_for: {
            "for" = 1
        }
    }
}
output simpleTemplate {
    value = aValue
}
output quotedTemplate {
    value = "The key is ${aKey}"
}
output heredoc {
    value = <<END
This is also a template.
So we can output the key again ${aKey}
END
}
output forTuple {
    value = [for key, value in ["a", "b"] : "${key}:${value}:${aValue}" if key != 0]
}
output forTupleValueOnly {
    value = [for value in ["a", "b"] : "${value}:${aValue}"]
}
output forTupleValueOnlyAttr {
    value = [for x in [{id="i-123",zone="us-west"},{id="i-abc",zone="us-east"}]: x.id if x.zone == "us-east"]
}
output forObject {
    value = {for key, value in ["a", "b"] : key => "${value}:${aValue}" if key != 0}
}
output relativeTraversalAttr {
    value = aListOfMaps[0].x
}
output relativeTraversalIndex {
    value = aListOfMaps[0]["x"]
}
output conditionalExpr {
    value = aValue == 0 ? "true" : "false"
}
