The `simulate` command manages project simulations. Simulations allow you to see
what changes would occur in a control plane after applying the latest version of
an Upbound project.

#### Examples

Create a new simulation for the specified control plane, wait for the simulation
to complete, then show results:

```shell
up project simulate create control-plane-name
```

Force completion of an in-progress project simulation (for example, because a
simulation is stuck or taking too long):

```shell
up project simulate complete simulation-name
```

Delete a simulation, removing the simulation results and resources:

```shell
up project simulate delete simulation-name
```
