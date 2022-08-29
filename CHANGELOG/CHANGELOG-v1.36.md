# Changelog v1.36

## [MALFORMED]


 - #2284 invalid type "fchore"
 - #2356 unknown section "monitoring"

## Know before update


 - All ingress nginx controllers with not-specified version (0.33) will restart and upgrade to 1.1
 - All of the controllers will be restarted.

## Features


 - **[candi]** Add an `additionalRolePolicies` parameter to AWSClusterConfiguration (#1005) [#2256](https://github.com/deckhouse/deckhouse/pull/2256)
 - **[candi]** Add support for the kubernetes 1.24 version [#2210](https://github.com/deckhouse/deckhouse/pull/2210)
 - **[candi]** Set `maxAllowed` and `minAllowed` to all VPA objects. Set resources requests for all controllers if VPA is off.   Added `global.modules.resourcesRequests.controlPlane` values. `global.modules.resourcesRequests.EveryNode` and `global.modules.resourcesRequests.masterNode` values are deprecated. [#1918](https://github.com/deckhouse/deckhouse/pull/1918)
    All of the controllers will be restarted.
 - **[ingress-nginx]** Change default ingress nginx controller version to 1.1 [#2267](https://github.com/deckhouse/deckhouse/pull/2267)
    All ingress nginx controllers with not-specified version (0.33) will restart and upgrade to 1.1
 - **[log-shipper]** Refactor transforms composition, improve efficiency and fix destination transforms. [#2050](https://github.com/deckhouse/deckhouse/pull/2050)
 - **[monitoring-kubernetes]** Add nodes count panel to the Nodes dashboard. [#2196](https://github.com/deckhouse/deckhouse/pull/2196)

## Fixes


 - **[ingress-nginx]** Change defaultControllerVersion without deckhouse reboot [#2338](https://github.com/deckhouse/deckhouse/pull/2338)
 - **[log-shipper]** Rewrite Elasticsearch dedot rule in VRL to improve performance. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[log-shipper]** Prevent Vector from stopping logs processing if Kubernetes API server was restarted. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[log-shipper]** Fix memory leak for internal metrics. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[node-manager]** Change cluster autoscaler timeouts to avoid node flapping [#2279](https://github.com/deckhouse/deckhouse/pull/2279)

## Chore


 - **[istio]** refactor istio revision monitoring [#2273](https://github.com/deckhouse/deckhouse/pull/2273)
 - **[log-shipper]** Update Vector to 0.23 [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[monitoring-kubernetes]** Bump kube-state-metrics 2.6.0 [#2291](https://github.com/deckhouse/deckhouse/pull/2291)

