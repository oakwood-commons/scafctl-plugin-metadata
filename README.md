# scafctl-plugin-metadata

Metadata provider plugin for scafctl - returns runtime metadata about the process and solution

## Installation

```bash
# Build from source
task build

# Or download from releases
gh release download --repo github.com/oakwood-commons/scafctl-plugin-metadata
```

## Usage

Register this plugin in your scafctl configuration, then reference
the **metadata** provider in your solutions:

```yaml
resolvers:
  my-value:
    resolve:
      with:
        - provider: metadata
          inputs:
            value: "hello"
```

## Development

```bash
# Run tests
task test

# Run linter
task lint

# Build
task build

# Full CI pipeline
task ci
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache-2.0 -- see [LICENSE](LICENSE) for details.
