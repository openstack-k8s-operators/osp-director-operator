Q: WHAT IS TESTED HERE?

A: 
- create osnetcfg with no static reservation
- add static reservation for controller-0
- scale up to 1 controller ctlplane
- Run webhook fail tests
  - add static reservation with wrong IP format
  - add static reservation with dupe IP
- add good additional reservation
- scale ctlplane nodes
- scale down node to verify reservation persist
- set persist to false to verify reservations for deleted nodes are gone
- add a new network
