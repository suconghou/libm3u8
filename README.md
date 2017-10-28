# libm3u8


init

```
m, err := libm3u8.NewFromURL(url, nil)
```

for file links

```
io.Copy(os.Stdout, m)
```

for stream download
```
io.Copy(os.Stdout, m.Play())
```
