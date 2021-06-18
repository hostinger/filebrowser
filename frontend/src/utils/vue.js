import Vue from "vue";
import Noty from "noty";
import VueLazyload from "vue-lazyload";
import i18n from "@/i18n";
import { disableExternal } from "@/utils/constants";

Vue.use(VueLazyload);

Vue.config.productionTip = true;

const notyDefault = {
  type: "info",
  layout: "bottomRight",
  timeout: 1000,
  progressBar: true,
};

Vue.prototype.$noty = (opts) => {
  new Noty(Object.assign({}, notyDefault, opts)).show();
};

Vue.prototype.$showSuccess = (message) => {
  new Noty(
    Object.assign({}, notyDefault, {
      text: message,
      type: "success",
    })
  ).show();
};

Vue.prototype.$showError = (error, displayReport = true) => {
  let btns = [
    Noty.button(i18n.t("buttons.close"), "", function () {
      n.close();
    }),
  ];

  if (!disableExternal && displayReport) {
    btns.unshift(
      Noty.button(i18n.t("buttons.reportIssue"), "", function () {
        window.open(
          "https://github.com/filebrowser/filebrowser/issues/new/choose"
        );
      })
    );
  }

  let message = error.message || error;

  if (isJSON(message)) {
    message = JSON.parse(message);
    if (message instanceof Object) {
      if (message["type"]) {
        message = i18n.t("errors." + message["type"]);
      } else {
        message = i18n.t("errors.internal");
      }
    }
  }

  let n = new Noty(
    Object.assign({}, notyDefault, {
      text: message,
      type: "error",
      timeout: null,
      buttons: btns,
    })
  );

  n.show();
};

Vue.directive("focus", {
  inserted: function (el) {
    el.focus();
  },
});

function isJSON(str) {
  try {
    return JSON.parse(str) && !!str;
  } catch (e) {
    return false;
  }
}

export default Vue;
