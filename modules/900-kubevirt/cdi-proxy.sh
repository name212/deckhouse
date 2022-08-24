kubectl patch cdi cdi \
  --type merge \
  --patch '{"spec":{"config":{"uploadProxyURLOverride":"https://cdi-uploadproxy.34.66.42.159.nip.io"}}}'
