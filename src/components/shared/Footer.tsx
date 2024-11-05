import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faGithub } from "@fortawesome/free-brands-svg-icons";

export const Footer = () => {
  return (
    <footer className="w-full">
      <div className="flex justify-center">
        <a
          href="https://github.com/autobrr/dashbrr"
          target="_blank"
          rel="noopener noreferrer"
          className="text-gray-400 hover:text-gray-300 transition-colors"
          title="View on GitHub"
        >
          <FontAwesomeIcon icon={faGithub} className="h-5 w-5" />
        </a>
      </div>
    </footer>
  );
};
