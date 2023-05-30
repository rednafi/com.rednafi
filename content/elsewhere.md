---
title: "Elsewhere"
summary: "Vendored timelines from twitter"
layout: "page"
hidemeta: true
ShowBreadCrumbs: false
---

<style>
.twitter-timeline, .tweets-skeleton{
  display: flex;
  flex-flow: row wrap;
  width: 100%;
  justify-content: left;
}
.twitter-timeline, .tweet-skeleton{
  width: 41rem;
}
.tweet-skeleton{
  border: 0.05rem solid rgb(190, 190, 190);
  border-radius: 1rem;
  height: 30rem;
  margin-bottom: 1rem;
  padding: 1.5rem;
}
.tweet-skeleton .img{
  height: 5rem;
  width: 5rem;
  border-radius: 50%;
  background-color: rgb(209, 209, 209);
}
.tweet-skeleton .content-1, .tweet-skeleton .content-2{
  height: 25%;
  margin-top: 1rem;
}
.tweet-skeleton .line{
  height: 15%;
  margin: 0.5rem 0;
  width: 100%;
  border-radius: 0.3rem;
  background-color: rgb(209, 209, 209);
}
.tweet-skeleton .line:last-child{
  width: 75%;
}
@keyframes tweet-skeleton {
  0%{
    background-color: rgb(209, 209, 209);
  }
  100%{
    background-color: rgb(243, 243, 243);
  }
}
</style>

<div class="tweets-skeleton">
  <div class="tweet-skeleton">
    <div class="img"></div>
    <div class="content-1">
      <div class="line"></div>
      <div class="line"></div>
      <div class="line"></div>
    </div>
    <div class="content-2">
      <div class="line"></div>
      <div class="line"></div>
    </div>
  </div>
</div>

<a class="twitter-timeline"
  style="visibility: hidden; display: none;"
  data-height="800"
  data-dnt="true"
  href="https://twitter.com/rednafi?ref_src=twsrc%5Etfw">
  Redowan's Tweets
</a>

<a
  class="twitter-timeline"
  style="visibility: hidden; display: none;"
  data-height="800"
  data-dnt="true"
  href="https://twitter.com/rednafi/lists/1663312088251465728?ref_src=twsrc%5Etfw">
  Redowan's Twitter List
</a>

<script>
window.twttr = (function (d, s, id) {
  let js,
    fjs = d.getElementsByTagName(s)[0],
    t = window.twttr || {};
  if (d.getElementById(id)) return t;
  js = d.createElement(s);
  js.id = id;
  js.src = "https://platform.twitter.com/widgets.js";
  fjs.parentNode.insertBefore(js, fjs);

  t._e = [];
  t.ready = function (f) {
    t._e.push(f);
  };

  return t;
})(document, "script", "twitter-wjs");
</script>

<script>
    const tweets_skeleton = document.querySelector(".tweets-skeleton");
    const tweet_skeleton = document.querySelector(".tweet-skeleton");

    for (let i = 0; i < 1; i++) {
        tweets_skeleton.append(tweet_skeleton.cloneNode(true));
    }

    setTimeout(() => {
      document.querySelector(".twitter-timeline").style = "visibility: hidden;";
      tweets_skeleton.style = "display: none;";
    }, 1000);
</script>
