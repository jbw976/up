The `simulation` command manages control plane simulations. Simulations allow
you to see what changes would occur in a control plane after applying a set of
changes.

#### Examples

Create a new simulation for the specified control plane, wait for the simulation
to complete, then shows results:

```shell
up controlplane simulation create control-plane-name
```

List all simulations for the current context:

```shell
up controlplane simulation list
```

Delete a simulation, removing the simulation results and resources:

```shell
up controlplane simulation delete simulation-name
```
