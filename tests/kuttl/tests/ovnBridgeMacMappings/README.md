Q: WHAT IS TESTED HERE?

A:
- create osnetcfg with no static reservation
- add static reservation for controller-0
- Run webhook fail tests
  - add static reservation with wrong MAC format
  - add static reservation with dupe MAC
  - change MAC of current reservation
- add good additional reservation
- scale ctlplane nodes
- scale down node to verify reservation persist
- set persist to false to verify reservations for deleted nodes are gone
- add new second physnet
