package html

// TODO: Can this be simpler for non-flipping postcards?
var htmlTmpl = `<input type="checkbox" id="postcard-{{.Name}}">
<label for="postcard-{{.Name}}">
	<div class="postcard {{ .Flip }} {{ if gt .FrontDimensions.PxHeight .FrontDimensions.PxWidth }}portrait{{ else }}landscape{{ end }}" style="--postcard: url('{{ .Name }}.webp'); --aspect-ratio: {{ .FrontDimensions.PxWidth }} / {{ .FrontDimensions.PxHeight }}">
		<img src="{{ .Name }}.webp" loading="lazy" alt="{{ .Front.Description }}" width="500px">
		<div class="shadow"></div>
	</div>
</label>`
