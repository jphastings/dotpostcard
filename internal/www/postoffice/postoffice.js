async function processResult(res) {
  const mimeType = res.headers.get('Content-Type').split(';')[0]
  if (mimeType == 'multipart/form-data') {
    const fd = await res.formData()
    displayPostcard(fd.values())
  } else {
    const blob = await res.blob()
    const file = new File([blob], extractFilename(res), { type: blob.type });
    downloadFile(file)
  }
}

function extractFilename(res) {
  const cdh = res.headers.get('Content-Disposition')
  const filenameRegex = /filename=(?:(["'])([^;\n]+?)\1)/i;
  const matches = filenameRegex.exec(cdh);
  if (!matches) {
    return
  }

  return matches[2]
}

async function onlyOk(res) {
  if (!res.ok) {
    const body = await res.text().catch()
    if (res.status == 400) {
      throw new Error(body)
    } else {
      throw new Error(`Unusable HTTP response (${res.status}): ${body}`)
    }
    
  }
  return res
}

async function displayPostcard(files) {
  let image, html

  for (file of files) {
    switch(file.type) {
      case "text/css":
        await file.text().then(insertPostcardCSS)
        break;
      case "text/html":
        html = await file.text()
        break;
      case "image/jpeg":
      case "image/png":
        image =  {
          filename: file.name,
          blobURL: URL.createObjectURL(file),
        }
        break;
    }
  }

  insertPostcardHTML(html, image)
  const outClass = document.querySelector('#output').classList
  outClass.toggle('loading', false)
}


function insertPostcardCSS(css) {
  const outCSS = document.querySelector('#output style')
  outCSS.appendChild(document.createTextNode(css));
}

function insertPostcardHTML(html, image) {
  const outHTML = document.querySelector('#output div.postcard-html')
  const outDownload = document.querySelector('#output a.postcard-download')

  const [_, uniqueHTML] = html.split("\n\n")
  if (!uniqueHTML) {
    throw new Error("Unexpected HTML returned from postcard generator")
  }
  // TODO: fix this hack to replace the filename
  const toReplace = image.filename.replace(/\.[^/.]+$/, '.jpeg')
  outHTML.innerHTML = uniqueHTML.replaceAll(toReplace, image.blobURL)
  outDownload.href = image.blobURL
  outDownload.download = image.filename
}


function downloadFile(file) {
  const url = URL.createObjectURL(file);

  const a = document.createElement('a');
  a.href = url;
  a.download = file.name;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function sideImageChanged(e) {
  const input = e.target
  const file = input.files[0]
  const reader = new FileReader();

  reader.addEventListener('load', (file) => {
    const img = new Image();

    img.addEventListener('load', () => {
      const width = img.width;
      const height = img.height;

      if (Math.abs(width - height) < 10) {
        input.dataset.orientation = "square"
      } else if (width > height) {
        input.dataset.orientation = "landscape"
      } else {
        input.dataset.orientation = "portrait"
      }

      input.setCustomValidity("")
      showAppropriateFlips()
    })

    img.addEventListener('error', () => {
      input.setCustomValidity("Doesn't appear to be an image file")
    })

    // Trigger loading the image
    img.src = file.target.result;
  })

  reader.addEventListener('error', () => {
    input.setCustomValidity("Couldn't read the chosen file")
  })
  
  reader.readAsDataURL(file)
}

function showAppropriateFlips() {
  const frontOrientation = document.getElementById('front-image')?.dataset?.orientation
  const backOrientation = document.getElementById('back-image')?.dataset?.orientation
  const flipChoices = document.querySelectorAll('input[name="flip"]')

  if (!frontOrientation) {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', true))
  } else if (!backOrientation) {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', flip.value != ""))
  } else if (frontOrientation == "square") {
    flipChoices.forEach((flip) => flip.parentElement.classList.toggle('irrelevant', false))
  } else {
    const homoriented = frontOrientation == backOrientation
    flipChoices.forEach((flip) => {
      const isIrrelevant = flip.value == '' || flip.value.endsWith('-hand') == homoriented
      flip.parentElement.classList.toggle('irrelevant', isIrrelevant)
    })
  }

  const chosen = document.querySelector('input[name="flip"]:checked')
  if (!chosen || chosen.parentElement.classList.contains('irrelevant')) {
    const first = document.querySelector('label:has(input[name="flip"]):not(.irrelevant) input')
    if (first) {
      first.checked = true
    }
  }
}

document.addEventListener("DOMContentLoaded", () => {
  const form = document.getElementById('postcard-input')
  const output = document.getElementById('output')
  
  form.addEventListener('submit', async (event) => {
    event.preventDefault()
    form.classList.toggle('irrelevant', true)
    output.classList.toggle('irrelevant', false)
    output.classList.toggle('loading', true)

    const {action, method} = event.target
    const body = new FormData(event.target)
  
    const res = await fetch(action, { method, body,})
      .then(onlyOk)
      .then(processResult)
  })

  document.getElementById('begin').addEventListener('click', () => {
    form.classList.toggle('irrelevant', false)
    form.classList.toggle('output', true)
  })

  // Now the submit listener is registered, swap to showing the postcard on the page, instead of downloading
  document.querySelector('input[name="codec-choice"][value="web"]').value = "web-js"

  document.querySelectorAll('#front-image,#back-image')
    .forEach((input) => {
      input.addEventListener('change', sideImageChanged)
    })
  showAppropriateFlips()
})

navigator.serviceWorker?.register('postoffice-serviceworker.js')?.catch(console.error)
