# Glimpse [![Build Status][1]][2]

Service directory for active & passive discovery on top of doozerd[3].

[1]: https://secure.travis-ci.org/soundcloud/glimpse.png
[2]: http://travis-ci.org/soundcloud/glimpse
[3]: https://github.com/ha/doozerd


## Documentation

We have a pretty good user documentation [here](http://go/service-discovery)
and some introduction slides [here](http://go/sd-slides).

See [the gopkgdoc page](http://gopkgdoc.appspot.com/github.com/soundcloud/glimpse)
for up-to-the-minute documentation and usage.


## Installing

Install [Go 1][4], either [from source][5] or [with a prepackaged binary][6].
Then,

```bash
$ go get github.com/soundcloud/glimpse
```

[4]: http://golang.org
[5]: http://golang.org/doc/install/source
[6]: http://golang.org/doc/install

## Contributing

Pull requests are very much welcomed.  Create your pull request on a non-master branch, make sure a test or example is included that covers your change and your commits represent coherent changes that include a reason for the change.

To run the integration tests, make sure you have Doozerd reachable under the [DefaultUri][7] and run `go test`. TravisCI will also run the integration tests.

[7]: https://github.com/soundcloud/glimpse/blob/master/glimpse.go#L11

## Credits

* [Alexander Simmerl][8]

[8]: https://github.com/xla

## License

BSD 2-Clause, see [LICENSE][9] for more details.

[9]: https://github.com/soundcloud/glimpse/blob/master/LICENSE
