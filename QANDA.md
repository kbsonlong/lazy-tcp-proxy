
# Questions and Answers

### Is there any way to find out what containers are managed?

> Only by looking at the logs.
>
> [Let me know if you would like an alternative!](/issues) (e.g. a dedicated HTTP endpoint that lists managed containers)

### What happens when there aren't enough resources?

> The app has no awareness of resource capacity. It will just try to start the requested service.
>
> [Let me know how if it should handle this scenario differently!](/issues)

### Is there an example setup I can try?

> Yes, please check the example docker compose under [example/docker-compose.yml](example/docker-compose.yml).

### What happens on container port conflicts?

> The app logs an error, and ignores the second conflicting container. 

### Can I run two instances of lazy-tcp-proxy?

> Yes, but it would be redundant. They would both just duplicate control. There is no way to scope the managed containers at this point.

### Does the app handle UDP Traffic?

> Not yet. But it could.
>
> [Let me know if you want this!](/issues)

### Does the app support calling webhooks?

> Not yet. But it could.
>
> [Let me know if you want this!](/issues)

### Does the app support Docker Swarm/Stacks?

> Not yet. But it could.
>
> It would be nice if it could also scale >1 if required.
>
> [Let me know if you want this!](/issues)

### Does the app support Kubernetes?

> Not yet. But it could.
>
> It would be nice if it could also scale >1 if required.
>
> [Let me know if you want this!](/issues)