---
title: Building a web app to display CSV file stats with ChatGPT & Observable
date: 2023-04-10
tags:
    - JavaScript
---

Whenever I plan to build something, I spend 90% of my time researching and figuring out the
idiosyncrasies of the tools that I decide to use for the project. LLM tools like ChatGPT has
helped me immensely in that regard. I'm taking on more tangential side projects because
they're no longer as time-consuming as they used to be and provide me with an immense amount
of joy and learning opportunities. While LLM interfaces like ChatGPT may hallucinate,
confabulate, and confidently give you misleading information, they also allow you to avoid
starting from scratch when you decide to work on something. Personally, this benefits me
enough to keep language models in my tool belt and use them to churn out more exploratory
work at a much faster pace.

For some strange reason, I never took the time to explore ObservableHQ[^1], despite knowing
what it does and how it can help me quickly build nifty client-side tools without going
through the hassle of containerizing and deploying them as dedicated applications. So, I
asked ChatGPT to build me a tool that would allow me to:

-   Upload two CSV files
-   Calculate the row and column counts from the files
-   Show the number of rows and columns in a table and include the headers of the columns
    and their corresponding index numbers, so that you can compare them easily.

Here's the initial prompt that I used:

> Give me the JavaScript code for an Observable notebook that'll allow me to upload a CSV
> file, calculate the row and column counts from it, and then display the stats with column
> headers and their corresponding index starting from 0. Display the info in an HTML table.

Then I refactored the JavaScript it returned so that it'll allow me to upload two CSV files
and compare their stats. I made ChatGPT do it for me with this follow-up prompt:

> Can you change the code so that it allows uploading two CSV files and displays the stats
> of both of them in two HTML tables? Don't blindly repeat the logic from the previous
> section twice.

Finally, I asyncified the code and changed some HTML parsing to make the table look a bit
better. Here's the complete 85-line code snippet:

```js
{
  // create file input elements for the two files
  const fileInput1 = html`<input type="file" />`;
  const fileInput2 = html`<input type="file" />`;

  // create empty HTML tables for the two files
  const table1 = html` <table>
    <thead>
      <tr>
        <th></th>
      </tr>
    </thead>
    <tbody></tbody>
  </table>`;
  const table2 = table1.cloneNode(true);

  // function to handle file load event and display stats in table
  const handleFileLoad = (table) => async (event) => {
    const file = event.target.files[0];
    const reader = new FileReader();

    // read the file contents as text
    reader.readAsText(file);

    // create a promise to wait for the file to load and parse
    const fileLoaded = new Promise((resolve, reject) => {
      reader.onload = () => {
        const contents = reader.result;
        const lines = contents.trim().split("\n");
        const headers = lines[0].split(",");
        const numColumns = headers.length;
        const numRows = lines.length - 1;

        // create a row for the number of rows
        const numRowsRow = html`<tr>
          <td>Number of rows:</td>
          <td>${numRows}</td>
        </tr>`;

        // create a row for the number of columns
        const numColsRow = html`<tr>
          <td>Number of columns:</td>
          <td>${numColumns}</td>
        </tr>`;

        // create a row for the column names
        const headerRow = html`<tr>
          <td>Column names:</td>
          <td>${headers.map((h, i) => `${i}: ${h}`).join(", ")}</td>
        </tr>`;

        // add the rows to the table body
        const tableBody = html`<tbody>
          ${numRowsRow}${numColsRow}${headerRow}
        </tbody>`;

        table.replaceChild(tableBody, table.lastChild);

        // resolve the promise with the parsed data
        resolve({ numRows, numColumns, headers });
      };

      reader.onerror = () => {
        reject(reader.error);
      };
    });

    // wait for the promise to resolve before displaying the results in the table
    try {
      const { numRows, numColumns, headers } = await fileLoaded;
      console.log(
        `File loaded: ${file.name},
        Rows: ${numRows}, Columns: ${numColumns}, Headers: ${headers}`
      );
    } catch (err) {
      console.error(err);
    }
  };

  // add event listeners to the file input elements
  fileInput1.addEventListener("change", handleFileLoad(table1));
  fileInput2.addEventListener("change", handleFileLoad(table2));

  // display the file input and table elements in the notebook
  return html`${fileInput1} ${table1} ${fileInput2} ${table2}`;
}
```

The snippet above starts by creating two file input elements using HTML input tags. These
are used to allow the user to select and upload CSV files. Two empty HTML tables are also
created to hold the extracted statistics for each CSV file.

Next, it defines a function called `handleFileLoad` which takes a table element as its
argument. This function is called when the user uploads a file, and it reads the contents of
the file and extracts some basic statistics from it. These statistics are then used to
populate the HTML table with the extracted information.

Inside the `handleFileLoad` function, the `FileReader` API is used to read the contents of
the uploaded file. The file contents are then parsed as text and split into lines. The first
line contains the column headers, which are extracted by splitting the line by commas. The
number of columns is then determined by the number of headers, and the number of rows is
determined by counting the number of lines in the file (excluding the header).

It then creates three rows for the extracted statistics: one row for the number of rows, one
row for the number of columns, and one row for the column headers with their corresponding
indexes starting from zero. The rows are then added to the HTML table.

Finally, the code adds event listeners to the file input elements to trigger the
`handleFileLoad` function when the user uploads a file. The file input elements and HTML
tables are then returned as an HTML fragment using the HTML template literal, and displayed
in the notebook.

You can find the working application embedded in the following section. Try uploading two
CSV files by clicking on the `Choose File` button and see how the app displays the stats in
separate HTML tables.

<iframe width="100%" height="500" frameborder="0"
  src="https://observablehq.com/embed/@rednafi/compare-two-csv-files@latest?cell=*">
</iframe>

Here's a gif of it in action:

<video width="100%" height="350" controls alt="observable notebook">
  <source
    src="https://user-images.githubusercontent.com/
30027932/231012563-375e07b6-9366-460b-9714-495b83a70b08.mov"
    type="video/mp4"
  />
</video>

Click on the following thumbnail to take the notebook for a spin:

[![observable thumbnail][image_1]][observable notebook]

[^1]: [ObservableHQ](https://observablehq.com/)
[^2]: [Observable notebook] [^2]

[image_1]:
    https://user-images.githubusercontent.com/30027932/231017828-91745232-d7ea-4573-86f6-341007fdc816.png
[observable notebook]: https://observablehq.com/@rednafi/compare-two-csv-files
