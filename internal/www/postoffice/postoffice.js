document.addEventListener("DOMContentLoaded", () => {
  document.getElementById('postcard-input').addEventListener('submit', async (event) => {
    event.preventDefault()
    const {action, method} = event.target
    const body = new FormData(event.target)
  
    const res = await fetch(action, { method, body,})
      .then(onlyOk)
      .then(processResult)
  })
})

async function processResult(res) {
  if (res.headers.get('Content-Type') == "multipart/form") {
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

function onlyOk(res) {
  if (!res.ok) {
    throw new Error(`Unusable HTTP response: ${res.res.statusText}`)
  }
  return res
}

async function displayPostcard(files) {
  let image, html

  for (file of files) {
    switch(file.type) {
      case "model/vnd.usdz+zip":
        // Shortcut and just download the USDZ here, for now
        downloadFile(file)
        return
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
  document.querySelector('#output').style.display = 'block'
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

navigator.serviceWorker.register('postoffice-serviceworker.js').catch(console.error)
