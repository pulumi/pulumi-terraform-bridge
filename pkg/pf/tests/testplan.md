This is the test plan for making sure PF diffs work correctly:

### Primitives

Schemas:
- Primitive without RequiresReplace
- Primitive with RequiresReplace
  
Scenarios:
- unchanged
- added
- removed
- changed

### Lists
Schemas:

1. List attribute
   - list attribute without RequiresReplace
   - list attribute with RequiresReplace
1. List nested attribute
   - list nested attribute without RequiresReplace
   - list nested attribute with RequiresReplace on the list
   - list nested attribute with RequiresReplace on the nested attribute
1. List nested block
   - list nested block without RequiresReplace
   - list nested block with RequiresReplace on the list
   - list nested block with RequiresReplace on the nested attribute

Scenarios:
- unchanged non-empty
- unchanged empty
- unchanged null
- changed non-empty
- added
- removed
- null to non-null
- non-null to null
- null to empty
- empty to null
- element added
- element removed


### Sets
Schemas:
1. Set attribute
   - set attribute without RequiresReplace
   - set attribute with RequiresReplace
1. Set nested attribute
   - set nested attribute without RequiresReplace
   - set nested attribute with RequiresReplace on the set
   - set nested attribute with RequiresReplace on the nested attribute
1. Set nested block
   - set nested block without RequiresReplace
   - set nested block with RequiresReplace on the set
   - set nested block with RequiresReplace on the nested attribute


Scenarios:
- unchanged non-empty
- unchanged empty
- unchanged null
- changed non-empty
- added
- removed
- null to non-null
- non-null to null
- null to empty
- empty to null
- element removed from front
- element removed from front unordered
- element removed from middle
- element removed from middle unordered
- element removed from end
- element removed from end unordered
- element added to front
- element added to front unordered
- element added to middle
- element added to middle unordered
- element added to end
- element added to end unordered
- elements shuffled unchanged
- elements shuffled unchanged unordered


### Maps
Schemas:
1. Map attribute
   - map attribute without RequiresReplace
   - map attribute with RequiresReplace
1. Map nested attribute
   - map nested attribute without RequiresReplace
   - map nested attribute with RequiresReplace on the map
   - map nested attribute with RequiresReplace on the nested attribute

Scenarios:
- unchanged non-empty
- unchanged empty
- unchanged null
- unchanged null value
- added
- removed
- changed value non-null
- changed value null to non-null
- changed value non-null to null
- changed key non-null value
- changed key null value
- key removed
- key added


### Objects
Schemas:
1. Object attribute
   - object attribute without RequiresReplace
   - object attribute with RequiresReplace
1. Single nested block
   - single nested block without RequiresReplace
   - single nested block with RequiresReplace on the object
   - single nested block with RequiresReplace on the nested attribute


Scenarios:
- unchanged null
- unchanged empty
- unchanged non-null
- changed non-null
- null to non-null
- non-null to null
- null to empty
- empty to null
- added
- removed
- changed value non-null