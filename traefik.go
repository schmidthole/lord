package main

var traefikDataDir = "/etc/traefik"

var traefikConfig = `
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

api:
  dashboard: true

certificatesResolvers:
  theresolver:
    acme:
      email: "%s" # config.CertEmail
      storage: "acme.json"
      httpChallenge:
        entryPoint: "web"

http:
  routers:
    dashboard:
      rule: "Host('%s')" # config.DashboardHostname
      service: "api@internal"
      entryPoints:
        - "websecure"
      tls:
        certResolver: "theresolver"
      middlewares:
        - "auth"
  middlewares:
    auth:
      basicAuth:
        users:
		# config.DashboardUsers will be appended here
`
