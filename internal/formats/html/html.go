package html

var htmlTmpl = `<div class="postcard {{ .Flip }} {{ if gt .FrontDimensions.PxHeight .FrontDimensions.PxWidth }}portrait{{ else }}landscape{{ end }}" style="--postcard: url('{{ .Filename }}'); --aspect-ratio: {{ .FrontDimensions.PxWidth }} / {{ .FrontDimensions.PxHeight }}">
  <img src="{{ .Filename }}" loading="lazy" alt="{{ .Front.Description }}" width="500px">
  <div class="shadow"></div>
</div>`
